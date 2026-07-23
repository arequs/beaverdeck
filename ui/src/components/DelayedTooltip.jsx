import React, { useCallback, useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';

export const ACTION_TOOLTIP_DELAY_MS = 400;

export default function DelayedTooltip({
  content,
  children,
  delayMs = ACTION_TOOLTIP_DELAY_MS
}) {
  const anchorRef = useRef(null);
  const timerRef = useRef(null);
  const [position, setPosition] = useState(null);
  const text = String(content || '').trim();

  const clearPending = useCallback(() => {
    if (timerRef.current != null) {
      window.clearTimeout(timerRef.current);
      timerRef.current = null;
    }
  }, []);

  const hide = useCallback(() => {
    clearPending();
    setPosition(null);
  }, [clearPending]);

  const scheduleShow = useCallback((immediate = false) => {
    clearPending();
    if (!text) return;

    const boundedDelay = immediate
      ? 0
      : Math.min(500, Math.max(0, Number(delayMs) || ACTION_TOOLTIP_DELAY_MS));
    timerRef.current = window.setTimeout(() => {
      timerRef.current = null;
      const rect = anchorRef.current?.getBoundingClientRect();
      if (!rect) return;

      const viewportWidth = document.documentElement.clientWidth || window.innerWidth;
      const halfWidth = Math.min(160, Math.max(0, (viewportWidth - 16) / 2));
      const center = Math.min(
        Math.max(rect.left + rect.width / 2, 8 + halfWidth),
        viewportWidth - 8 - halfWidth
      );
      const placement = rect.top >= 72 ? 'top' : 'bottom';
      setPosition({
        left: center,
        top: placement === 'top' ? rect.top - 7 : rect.bottom + 7,
        placement
      });
    }, boundedDelay);
  }, [clearPending, delayMs, text]);

  useEffect(() => () => clearPending(), [clearPending]);

  useEffect(() => {
    if (!position) return undefined;
    window.addEventListener('resize', hide);
    window.addEventListener('scroll', hide, true);
    return () => {
      window.removeEventListener('resize', hide);
      window.removeEventListener('scroll', hide, true);
    };
  }, [hide, position]);

  return (
    <span
      className="delayed-tooltip-anchor"
      ref={anchorRef}
      onMouseEnter={() => scheduleShow(false)}
      onMouseLeave={hide}
      onFocus={() => scheduleShow(true)}
      onBlur={hide}
      onMouseDown={hide}
    >
      {children}
      {position && text ? createPortal(
        <span
          className={`delayed-tooltip delayed-tooltip-${position.placement}`}
          role="tooltip"
          style={{ left: position.left, top: position.top }}
        >
          {text}
        </span>,
        document.body
      ) : null}
    </span>
  );
}
