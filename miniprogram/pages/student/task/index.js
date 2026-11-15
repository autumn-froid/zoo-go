const {request, q} = require('../../../utils/api')
const {decorateTask, money, fmtTime, tapFeedback} = require('../../../utils/format')

Page({
    data: {
        id: '',
        loading: true,
        submitting: false,
        task: null,
        price: 0,
        message: '',
        feeText: '0',
        receiveText: '0',
        canQuote: false,
        identityVerified: false,
        agreement: null,
        agreementVersion: '',
        identityConfirmed: false,
        visitRecordConfirmed: false,
        agreementAccepted: false,
        quoteTitle: '提交保密报价',
        navTitle: '任务详情',
    },

    onLoad(options) {
        if (!options.id) {
            wx.showToast({title: '缺少任务编号', icon: 'none'})
            return
        }
        this.setData({id: options.id})
    },

    onShow() {
        if (this.data.id) {
            this.loadTask()
            this.startTimer()
        }
    },

    onHide() {
        this.stopTimer()
    },

    onUnload() {
        this.stopTimer()
    },

    async loadTask() {
        try {
            const [task, agreement, identity] = await Promise.all([
                request(`/v1/tasks/${this.data.id}?${q({role: 'student'})}`),
                request('/v1/agreements/current?role=sitter'),
                request('/v1/identity/verification'),
            ])
            const decorated = decorateTask(task)
            const verification = identity.verification || {}
            const defaultPrice = task.myBid
                ? task.myBid.priceFen / 100
                : task.mode === 'fixed'
                    ? task.budgetFen / 100
                    : Math.max(20, Math.round(task.budgetFen * 0.8 / 100))
            this.setData({
                loading: false,
                task: decorated,
                price: defaultPrice,
                message: task.myBid ? task.myBid.message : '',
                quoteTitle: task.myBid ? '更新报价' : task.mode === 'fixed' ? '接受一口价' : '提交保密报价',
                canQuote: task.status === 'BIDDING' && task.remainingSec > 0,
                navTitle: task.status === 'BIDDING' ? '任务详情' : '任务已成交',
                identityVerified: verification.status === 'VERIFIED',
                agreement,
                agreementVersion: agreement.version,
            })
            this.updateEstimate(defaultPrice)
        } catch (error) {
            this.setData({loading: false})
            wx.showToast({title: error.message, icon: 'none'})
        }
    },

    startTimer() {
        this.stopTimer()
        this.timer = setInterval(() => {
            const task = this.data.task
            if (!task || task.status !== 'BIDDING') {
                return
            }
            const remainingSec = Math.max(0, task.remainingSec - 1)
            this.setData({
                'task.remainingSec': remainingSec,
                'task.remainingText': fmtTime(remainingSec),
                canQuote: remainingSec > 0,
            })
        }, 1000)
    },

    stopTimer() {
        if (this.timer) {
            clearInterval(this.timer)
            this.timer = null
        }
    },

    changePrice(event) {
        const delta = Number(event.currentTarget.dataset.delta)
        const price = Math.max(20, this.data.price + delta)
        this.setData({price})
        this.updateEstimate(price)
    },

    onPriceInput(event) {
        const price = Number(event.detail.value) || 0
        this.setData({price: event.detail.value})
        this.updateEstimate(price)
    },

    updateEstimate(price) {
        const cents = Math.round((Number(price) || 0) * 100)
        const fee = Math.floor(cents * 0.1)
        this.setData({feeText: money(fee), receiveText: money(cents - fee)})
    },

    onMessageInput(event) {
        this.setData({message: event.detail.value})
    },

    toggleConsent(event) {
        const field = event.currentTarget.dataset.field
        if (field) {
            this.setData({[field]: !this.data[field]})
        }
    },

    goIdentity() {
        wx.navigateTo({url: '/pages/identity/index'})
    },

    showAgreement() {
        wx.navigateTo({url: '/pages/agreement/index?role=sitter'})
    },

    async submitBid() {
        if (!this.data.task || !this.data.canQuote || this.data.submitting) {
            return
        }
        if (!this.data.identityVerified) {
            wx.showModal({
                title: '请先完成实名认证',
                content: '报价前需要核验真实身份。实名资料审核通过后即可报价。',
                confirmText: '去认证',
                success: (result) => {
                    if (result.confirm) {
                        this.goIdentity()
                    }
                },
            })
            return
        }
        if (!this.data.identityConfirmed || !this.data.visitRecordConfirmed || !this.data.agreementAccepted) {
            wx.showToast({title: '请逐项确认实名、上门记录和服务协议', icon: 'none'})
            return
        }
        if (!this.data.agreementVersion) {
            wx.showToast({title: '协议版本加载失败，请刷新后重试', icon: 'none'})
            return
        }
        const price = Number(this.data.price)
        const budget = this.data.task.budgetFen / 100
        if (!price || price < 20 || price > budget) {
            wx.showToast({title: `报价需在 ¥20–¥${budget} 之间`, icon: 'none'})
            return
        }
        tapFeedback()
        this.setData({submitting: true})
        try {
            await request(`/v1/tasks/${this.data.id}/bids`, {
                method: 'POST',
                data: {
                    priceFen: Math.round(price * 100),
                    message: this.data.message,
                    agreementVersion: this.data.agreementVersion,
                    agreementItems: ['sitter_identity_truth', 'sitter_visit_record', 'service_agreement'],
                    agreementAccepted: true,
                },
            })
            wx.showToast({title: this.data.task.myBid ? '报价已更新' : '报价已提交', icon: 'success'})
            await this.loadTask()
        } catch (error) {
            wx.showToast({title: error.message, icon: 'none'})
        } finally {
            this.setData({submitting: false})
        }
    },

    goFulfill() {
        if (this.data.task.orderId) {
            wx.navigateTo({url: `/pages/student/fulfill/index?id=${this.data.task.orderId}`})
        }
    },

    goHall() {
        wx.reLaunch({url: '/pages/student/hall/index'})
    },
})
