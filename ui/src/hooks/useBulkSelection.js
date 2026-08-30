import { useEffect, useMemo, useState } from 'react';

export default function useBulkSelection(items, getKey) {
  const [selectedKeys, setSelectedKeys] = useState([]);
  const availableKeys = useMemo(() => items.map((item) => getKey(item)), [items, getKey]);
  const availableKeySet = useMemo(() => new Set(availableKeys), [availableKeys]);
  const selectedKeySet = useMemo(() => new Set(selectedKeys), [selectedKeys]);
  const selectedItems = useMemo(
    () => items.filter((item) => selectedKeySet.has(getKey(item))),
    [items, getKey, selectedKeySet]
  );
  const allVisibleSelected = availableKeys.length > 0 && availableKeys.every((key) => selectedKeySet.has(key));
  const someVisibleSelected = availableKeys.some((key) => selectedKeySet.has(key));

  useEffect(() => {
    setSelectedKeys((previous) => {
      const next = previous.filter((key) => availableKeySet.has(key));
      return next.length === previous.length ? previous : next;
    });
  }, [availableKeySet]);

  function toggle(item) {
    const key = getKey(item);
    setSelectedKeys((previous) => (
      previous.includes(key)
        ? previous.filter((candidate) => candidate !== key)
        : [...previous, key]
    ));
  }

  function setAllVisible(shouldSelect) {
    setSelectedKeys((previous) => {
      const next = new Set(previous);
      availableKeys.forEach((key) => {
        if (shouldSelect) next.add(key);
        else next.delete(key);
      });
      return Array.from(next);
    });
  }

  function keep(itemsToKeep) {
    setSelectedKeys(itemsToKeep.map((item) => getKey(item)));
  }

  return {
    visibleCount: availableKeys.length,
    count: selectedItems.length,
    selectedItems,
    selectedKeySet,
    allVisibleSelected,
    someVisibleSelected,
    toggle,
    setAllVisible,
    keep
  };
}
