package main

import (
	"context"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"

	"zoo-go/internal/contentsecurity"
	"zoo-go/internal/httpapi"
	"zoo-go/internal/moderation"
	"zoo-go/internal/observability"
	"zoo-go/internal/payment"
	"zoo-go/internal/security"
	"zoo-go/internal/store"
)

func main() {
	addr := flag.String("addr", env("ADDR", ":8080"), "HTTP listen address")
	dataFile := flag.String("data", env("DATA_FILE", "data/zoo.db"), "SQLite database file; use empty string for memory only")
	uploadDir := flag.String("uploads", env("UPLOAD_DIR", "data/uploads"), "local upload directory")
	adminDir := flag.String("admin", env("ADMIN_DIR", "admin"), "admin web directory")
	reset := flag.Bool("reset", false, "delete the local database before starting")
	flag.Parse()

	if *reset && *dataFile != "" {
		for _, file := range []string{*dataFile, *dataFile + "-wal", *dataFile + "-shm"} {
			if err := os.Remove(file); err != nil && !os.IsNotExist(err) {
				slog.Error("reset database", "error", err)
				os.Exit(1)
			}
		}
	}

	logger, otelShutdown, err := observability.Setup(context.Background())
	if err != nil {
		slog.Error("initialize otel", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)
	defer func() {
		if err := observability.ShutdownWithTimeout(otelShutdown, 5*time.Second); err != nil {
			slog.Error("otel shutdown", "error", err)
		}
	}()

	piiCipher, err := security.NewCipher(os.Getenv("PII_ENCRYPTION_KEY"), os.Getenv("PII_HASH_KEY"))
	if err != nil {
		logger.Error("initialize PII encryption; set PII_ENCRYPTION_KEY and PII_HASH_KEY", "error", err)
		os.Exit(1)
	}
	st, err := store.NewWithOptions(*dataFile, store.Options{PII: piiCipher})
	if err != nil {
		logger.Error("initialize store", "error", err)
		os.Exit(1)
	}
	defer st.Close()

	ugcPolicy := moderation.DefaultPolicy()
	ugcPolicy.AutoApproveText = envBool("UGC_AUTO_APPROVE_TEXT", false)
	ugcPolicy.RequireImageReview = envBool("UGC_REQUIRE_IMAGE_REVIEW", true)
	if raw := os.Getenv("UGC_BLOCKED_TERMS"); raw != "" {
		for _, term := range strings.Split(raw, ",") {
			if term = strings.TrimSpace(term); term != "" {
				ugcPolicy.BlockedTerms = append(ugcPolicy.BlockedTerms, term)
			}
		}
	}
	st.SetModerationPolicy(ugcPolicy)

	var contentSecurityClient contentsecurity.Client
	var callbackVerifier contentsecurity.NotificationVerifier
	if envBool("UGC_MACHINE_REVIEW_ENABLED", false) {
		client, err := contentsecurity.NewWeChatClient(contentsecurity.Config{
			AppID: os.Getenv("WECHAT_APP_ID"), AppSecret: os.Getenv("WECHAT_APP_SECRET"),
		})
		if err != nil {
			logger.Error("initialize wechat content security", "error", err)
			os.Exit(1)
		}
		contentSecurityClient = client
		callbackToken := strings.TrimSpace(os.Getenv("WECHAT_CONTENT_SECURITY_TOKEN"))
		if callbackToken == "" {
			logger.Warn("WECHAT_CONTENT_SECURITY_TOKEN is not set; image review callbacks cannot be verified")
		} else {
			verifier, err := contentsecurity.NewVerifier(contentsecurity.CallbackConfig{
				Token:          callbackToken,
				EncodingAESKey: os.Getenv("WECHAT_CONTENT_SECURITY_AES_KEY"),
				AppID:          os.Getenv("WECHAT_APP_ID"),
			})
			if err != nil {
				logger.Error("initialize wechat content security callback", "error", err)
				os.Exit(1)
			}
			callbackVerifier = verifier
		}
		publicBaseURL := strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL"))
		if publicBaseURL == "" {
			logger.Warn("PUBLIC_BASE_URL is not set; image machine review will fall back to manual review")
		} else if !strings.HasPrefix(publicBaseURL, "https://") && !strings.HasPrefix(publicBaseURL, "http://") {
			logger.Error("PUBLIC_BASE_URL must be an absolute http(s) URL")
			os.Exit(1)
		}
		logger.Info("wechat content security enabled")
	} else {
		logger.Info("wechat content security disabled; UGC remains on local rules and manual review")
	}

	apiConfig := httpapi.Config{
		DevAuth:          envBool("DEV_AUTH", false),
		WeChatAppID:      os.Getenv("WECHAT_APP_ID"),
		WeChatAppSecret:  os.Getenv("WECHAT_APP_SECRET"),
		AdminToken:       os.Getenv("ADMIN_TOKEN"),
		UploadDir:        *uploadDir,
		AdminDir:         *adminDir,
		MediaSigningKey:  os.Getenv("UPLOAD_SIGNING_KEY"),
		PublicBaseURL:    strings.TrimSpace(os.Getenv("PUBLIC_BASE_URL")),
		ContentSecurity:  contentSecurityClient,
		CallbackVerifier: callbackVerifier,
	}
	payments, err := buildPaymentService(st, logger)
	if err != nil {
		logger.Error("initialize wechat pay", "error", err)
		os.Exit(1)
	}
	apiConfig.Payments = payments

	appCtx, cancelApp := context.WithCancel(context.Background())
	defer cancelApp()
	go runMachineReviewExpiry(appCtx, st, logger)
	if payments != nil {
		go payments.Run(appCtx, 30*time.Second)
		logger.Info("wechat pay enabled", "maintenance_interval", "30s")
	} else {
		logger.Warn("wechat pay disabled; payment endpoints will return PAYMENT_NOT_CONFIGURED")
	}

	handler := otelhttp.NewHandler(httpapi.New(st, apiConfig, logger).Handler(), "zoo-api")
	server := &http.Server{
		Addr:              *addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       60 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		logger.Info("pet feeding API started", "addr", *addr, "data", *dataFile)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server stopped", "error", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	cancelApp()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown", "error", err)
	}
}

func runMachineReviewExpiry(ctx context.Context, st *store.Store, logger *slog.Logger) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-ticker.C:
			expired, err := st.ExpireMachineReviews(now)
			if err != nil {
				logger.Error("expire machine media reviews", "error", err)
				continue
			}
			if expired > 0 {
				logger.Info("machine media reviews expired", "count", expired)
			}
		}
	}
}

func buildPaymentService(st *store.Store, logger *slog.Logger) (*payment.Service, error) {
	values := []string{
		os.Getenv("WECHAT_MCH_ID"),
		os.Getenv("WECHAT_PAY_API_V3_KEY"),
		os.Getenv("WECHAT_PAY_SERIAL_NO"),
		os.Getenv("WECHAT_PAY_PRIVATE_KEY_PATH"),
		os.Getenv("WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH"),
		os.Getenv("WECHAT_PAY_PLATFORM_SERIAL_NO"),
		os.Getenv("WECHAT_PAY_NOTIFY_URL"),
		os.Getenv("WECHAT_REFUND_NOTIFY_URL"),
	}
	configured := false
	for _, value := range values {
		if value != "" {
			configured = true
			break
		}
	}
	if !configured {
		return nil, nil
	}
	client, err := payment.NewClient(payment.Config{
		AppID:                  os.Getenv("WECHAT_APP_ID"),
		MchID:                  os.Getenv("WECHAT_MCH_ID"),
		MerchantSerialNo:       os.Getenv("WECHAT_PAY_SERIAL_NO"),
		APIv3Key:               os.Getenv("WECHAT_PAY_API_V3_KEY"),
		MerchantPrivateKeyPath: os.Getenv("WECHAT_PAY_PRIVATE_KEY_PATH"),
		PlatformPublicKeyPath:  os.Getenv("WECHAT_PAY_PLATFORM_PUBLIC_KEY_PATH"),
		PlatformSerialNo:       os.Getenv("WECHAT_PAY_PLATFORM_SERIAL_NO"),
		PaymentNotifyURL:       os.Getenv("WECHAT_PAY_NOTIFY_URL"),
		RefundNotifyURL:        os.Getenv("WECHAT_REFUND_NOTIFY_URL"),
	})
	if err != nil {
		return nil, err
	}
	return payment.NewService(st, client, logger), nil
}

func env(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envBool(key string, fallback bool) bool {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value == "1" || value == "true" || value == "TRUE" || value == "yes"
}
