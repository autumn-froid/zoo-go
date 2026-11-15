"use client";

import { motion, useReducedMotion, type HTMLMotionProps } from "motion/react";
import type { ReactNode } from "react";

const easeOut = [0.23, 1, 0.32, 1] as const;

const viewport = { once: true, amount: 0.22, margin: "0px 0px -40px 0px" };

type RevealProps = {
  children: ReactNode;
  className?: string;
  delay?: number;
} & Omit<HTMLMotionProps<"div">, "children">;

/** Soft rise only — never fades content out (avoids stuck opacity:0). */
export function Reveal({
  children,
  className,
  delay = 0,
  ...props
}: RevealProps) {
  const reduceMotion = useReducedMotion();

  return (
    <motion.div
      className={className}
      initial={reduceMotion ? false : { y: 12 }}
      whileInView={{ y: 0 }}
      viewport={viewport}
      transition={{ duration: 0.45, ease: easeOut, delay }}
      {...props}
    >
      {children}
    </motion.div>
  );
}

export function RevealItem({
  children,
  className,
  delay = 0,
}: {
  children: ReactNode;
  className?: string;
  delay?: number;
}) {
  const reduceMotion = useReducedMotion();

  return (
    <motion.div
      className={className}
      initial={reduceMotion ? false : { y: 16 }}
      whileInView={{ y: 0 }}
      viewport={viewport}
      transition={{ duration: 0.5, ease: easeOut, delay }}
    >
      {children}
    </motion.div>
  );
}

export { easeOut, viewport };
