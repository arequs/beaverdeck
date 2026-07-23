import React, { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { LockKeyhole, MoreHorizontal } from 'lucide-react';
import DelayedTooltip from './DelayedTooltip.jsx';

export default function ActionMenu({ actions = [] }) {
  const normalized = actions.filter((action) => action && action.label);
  const hasAnyAction = normalized.length > 0;
  const hasEnabled = normalized.some((action) => action.enabled);
  const disabledReasons = normalized
    .filter((action) => !action.enabled && action.reason)
    .map((action) => `${action.label}: ${action.reason}`);
  const [open, setOpen] = useState(false);
  const rootRef = useRef(null);
  const popoverRef = useRef(null);
  const [popoverStyle, setPopoverStyle] = useState({ top: 0, left: 0 });

  useEffect(() => {
    if (!open) return undefined;

    const updatePosition = () => {
      const rect = rootRef.current?.getBoundingClientRect();
      if (!rect) return;
      const menuWidth = 160;
      const menuHeight = Math.max(44, normalized.length * 38 + 8);
      const openUp = window.innerHeight - rect.bottom < menuHeight + 12;
      const left = Math.min(Math.max(8, rect.right - menuWidth), window.innerWidth - menuWidth - 8);
      const top = openUp
        ? Math.max(8, rect.top - menuHeight - 4)
        : Math.min(window.innerHeight - menuHeight - 8, rect.bottom + 4);
      setPopoverStyle({ top, left });
    };

    updatePosition();
    window.addEventListener('resize', updatePosition);
    window.addEventListener('scroll', updatePosition, true);
    return () => {
      window.removeEventListener('resize', updatePosition);
      window.removeEventListener('scroll', updatePosition, true);
    };
  }, [open, normalized.length]);

  useEffect(() => {
    if (!open) return undefined;
    const onDocClick = (event) => {
      if (!rootRef.current?.contains(event.target) && !popoverRef.current?.contains(event.target)) {
        setOpen(false);
      }
    };
    document.addEventListener('mousedown', onDocClick);
    return () => document.removeEventListener('mousedown', onDocClick);
  }, [open]);

  const run = (action) => {
    if (!action?.enabled) return;
    setOpen(false);
    if (action.onClick) action.onClick();
  };

  if (!hasAnyAction) {
    return <span>-</span>;
  }

  return (
    <div className="action-menu" ref={rootRef} onClick={(event) => event.stopPropagation()}>
      <DelayedTooltip content={hasEnabled ? 'Actions' : (disabledReasons.join('\n') || 'No actions available')}>
        <button
          className={`action-menu-trigger ${!hasEnabled ? 'disabled' : ''}`}
          onClick={() => hasEnabled && setOpen((value) => !value)}
          disabled={!hasEnabled}
          aria-label="Actions"
        >
          <MoreHorizontal size={15} strokeWidth={1.8} aria-hidden="true" />
        </button>
      </DelayedTooltip>
      {open ? createPortal(
        <div className="action-menu-popover" ref={popoverRef} style={popoverStyle}>
          {normalized.map((action) => (
            <DelayedTooltip key={action.label} content={action.enabled ? action.label : action.reason}>
              <button
                onClick={() => run(action)}
                disabled={!action.enabled}
              >
                <span>{action.label}</span>
                {!action.enabled ? <LockKeyhole size={13} strokeWidth={1.8} aria-hidden="true" /> : null}
              </button>
            </DelayedTooltip>
          ))}
        </div>,
        document.body
      ) : null}
    </div>
  );
}
