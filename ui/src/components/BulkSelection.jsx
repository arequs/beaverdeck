import React, { useEffect, useMemo, useRef } from 'react';

export function BulkSelectAll({ selection, label }) {
  const inputRef = useRef(null);

  useEffect(() => {
    if (inputRef.current) {
      inputRef.current.indeterminate = selection.someVisibleSelected && !selection.allVisibleSelected;
    }
  }, [selection.someVisibleSelected, selection.allVisibleSelected]);

  return (
    <input
      ref={inputRef}
      type="checkbox"
      checked={selection.allVisibleSelected}
      disabled={selection.visibleCount === 0}
      onChange={(event) => selection.setAllVisible(event.target.checked)}
      aria-label={selection.allVisibleSelected ? `Unselect all visible ${label}` : `Select all visible ${label}`}
    />
  );
}

export function BulkRowSelect({ selection, item, itemKey, label }) {
  const checked = selection.selectedKeySet.has(itemKey);
  return (
    <input
      type="checkbox"
      checked={checked}
      onChange={() => selection.toggle(item)}
      aria-label={`Select ${label}`}
    />
  );
}

function actionPermission(items, getPermission, verb) {
  if (items.length === 0) {
    return { allowed: false, reason: `Select at least one resource to ${verb.toLowerCase()}` };
  }
  for (const item of items) {
    const permission = getPermission(item);
    if (!permission.allowed) {
      return permission;
    }
  }
  return { allowed: true, reason: '' };
}

export function BulkActionButton({
  selection,
  verb,
  className = '',
  getPermission,
  runItem,
  refreshAll,
  safe
}) {
  const permission = useMemo(
    () => actionPermission(selection.selectedItems, getPermission, verb),
    [selection.selectedItems, getPermission, verb]
  );

  async function run() {
    const failed = [];
    for (const item of selection.selectedItems) {
      try {
        // Keep mutations sequential to avoid hammering the Kubernetes API server.
        // eslint-disable-next-line no-await-in-loop
        await runItem(item);
      } catch (error) {
        failed.push({ item, message: error.message || String(error) });
      }
    }
    selection.keep(failed.map((entry) => entry.item));
    await refreshAll();
    if (failed.length > 0) {
      throw new Error(`${verb} finished with ${failed.length} error(s): ${failed[0].message}`);
    }
  }

  return (
    <button
      type="button"
      className={className}
      disabled={!permission.allowed}
      title={permission.reason}
      onClick={() => safe(run)}
    >
      {verb}
    </button>
  );
}

export function BulkSelectionCount({ selection }) {
  return selection.count > 0 ? <span className="small-hint">{selection.count} selected</span> : null;
}
