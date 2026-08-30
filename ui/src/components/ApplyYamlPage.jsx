import React, { useEffect, useRef, useState } from 'react';
import { createPortal } from 'react-dom';
import { Check, ChevronDown, FileCode2 } from 'lucide-react';

function ApplyTemplatePicker({ selectedTemplate, loadTemplate, applyTemplates }) {
  const [open, setOpen] = useState(false);
  const triggerRef = useRef(null);
  const popoverRef = useRef(null);
  const [popoverStyle, setPopoverStyle] = useState({ top: 0, left: 0, width: 260 });
  const templateNames = Object.keys(applyTemplates);

  useEffect(() => {
    if (!open) return undefined;

    const updatePosition = () => {
      const rect = triggerRef.current?.getBoundingClientRect();
      if (!rect) return;
      const viewportWidth = document.documentElement.clientWidth || window.innerWidth;
      const viewportHeight = document.documentElement.clientHeight || window.innerHeight;
      const width = Math.min(280, viewportWidth - 16);
      const estimatedHeight = Math.min(360, viewportHeight - 16, 42 + templateNames.length * 31);
      const openUp = viewportHeight - rect.bottom < estimatedHeight + 12 && rect.top > estimatedHeight;
      const left = Math.min(Math.max(8, rect.left), viewportWidth - width - 8);
      const top = openUp
        ? Math.max(8, rect.top - estimatedHeight - 4)
        : Math.max(8, Math.min(viewportHeight - estimatedHeight - 8, rect.bottom + 4));
      setPopoverStyle({ top, left, width });
    };
    const closeOnEscape = (event) => {
      if (event.key === 'Escape') setOpen(false);
    };
    const closeOnOutsideClick = (event) => {
      if (!triggerRef.current?.contains(event.target) && !popoverRef.current?.contains(event.target)) {
        setOpen(false);
      }
    };

    updatePosition();
    window.addEventListener('resize', updatePosition);
    window.addEventListener('scroll', updatePosition, true);
    document.addEventListener('keydown', closeOnEscape);
    document.addEventListener('mousedown', closeOnOutsideClick);
    return () => {
      window.removeEventListener('resize', updatePosition);
      window.removeEventListener('scroll', updatePosition, true);
      document.removeEventListener('keydown', closeOnEscape);
      document.removeEventListener('mousedown', closeOnOutsideClick);
    };
  }, [open, templateNames.length]);

  const selectTemplate = (name) => {
    loadTemplate(name);
    setOpen(false);
  };

  return (
    <div className="apply-template-picker">
      <button
        ref={triggerRef}
        type="button"
        className={`apply-template-trigger ${open ? 'active' : ''}`.trim()}
        onClick={() => setOpen((value) => !value)}
        aria-haspopup="listbox"
        aria-expanded={open}
      >
        <FileCode2 size={15} strokeWidth={1.8} aria-hidden="true" />
        <span>{selectedTemplate || 'Load template...'}</span>
        <ChevronDown size={14} strokeWidth={1.8} aria-hidden="true" />
      </button>
      {open ? createPortal(
        <div
          ref={popoverRef}
          className="apply-template-popover"
          style={popoverStyle}
          role="listbox"
          aria-label="YAML templates"
        >
          <div className="apply-template-popover-head">
            <span className="small-label">YAML Templates</span>
            <span>{templateNames.length}</span>
          </div>
          <div className="apply-template-options">
            {templateNames.map((name) => {
              const selected = name === selectedTemplate;
              return (
                <button
                  key={name}
                  type="button"
                  className={selected ? 'active' : ''}
                  onClick={() => selectTemplate(name)}
                  role="option"
                  aria-selected={selected}
                >
                  <span>{name}</span>
                  {selected ? <Check size={14} strokeWidth={2} aria-hidden="true" /> : null}
                </button>
              );
            })}
          </div>
        </div>,
        document.body
      ) : null}
    </div>
  );
}

export default function ApplyYamlPage({
  selectedTemplate,
  loadTemplate,
  applyTemplates,
  yamlText,
  setYamlText,
  safe,
  applyYaml,
  primaryNamespace,
  permissionInfo
}) {
  const applyPermission = [
    { allowed: Boolean(primaryNamespace), reason: 'Select namespace first' },
    permissionInfo('apply', 'edit', primaryNamespace)
  ].find((item) => !item.allowed) || { allowed: true, reason: '' };

  return (
    <>
      <div className="toolbar fixed-toolbar">
        <ApplyTemplatePicker
          selectedTemplate={selectedTemplate}
          loadTemplate={loadTemplate}
          applyTemplates={applyTemplates}
        />
      </div>
      <textarea
        className="code-textarea"
        rows={12}
        value={yamlText}
        onChange={(e) => setYamlText(e.target.value)}
        placeholder={'---\napiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: demo'}
      />
      <div className="toolbar compact fixed-toolbar">
        <button className="warn" onClick={() => safe(() => applyYaml(true))} disabled={!applyPermission.allowed} title={applyPermission.reason}>
          Dry-run
        </button>
        <button onClick={() => safe(() => applyYaml(false))} disabled={!applyPermission.allowed} title={applyPermission.reason}>
          Apply
        </button>
      </div>
    </>
  );
}
