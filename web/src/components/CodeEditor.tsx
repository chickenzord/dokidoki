import React, { useMemo } from 'react';
import CodeMirror, { Extension } from '@uiw/react-codemirror';
import { yaml } from '@codemirror/lang-yaml';
import { json } from '@codemirror/lang-json';
import { StreamLanguage, indentUnit } from '@codemirror/language';
import { properties } from '@codemirror/legacy-modes/mode/properties';
import { shell } from '@codemirror/legacy-modes/mode/shell';
import { oneDark } from '@codemirror/theme-one-dark';
import { EditorView } from '@codemirror/view';

export interface CodeEditorProps {
  value: string;
  onChange?: (val: string) => void;
  filename?: string;
  readOnly?: boolean;
  height?: string;
  minHeight?: string;
  maxHeight?: string;
  className?: string;
  autoFocus?: boolean;
  placeholder?: string;
}

/**
 * Returns the appropriate CodeMirror language extension based on filename or content.
 */
function getLanguageExtension(filename?: string, content?: string): Extension[] {
  const name = (filename || '').toLowerCase();

  // YAML: compose files, yaml, yml
  if (
    name.endsWith('.yaml') ||
    name.endsWith('.yml') ||
    name.includes('compose')
  ) {
    return [yaml()];
  }

  // JSON
  if (name.endsWith('.json')) {
    return [json()];
  }

  // Environment / properties files: .env, .env.local, config.properties
  if (name.startsWith('.env') || name.endsWith('.env') || name.endsWith('.properties') || name.endsWith('.ini')) {
    return [StreamLanguage.define(properties)];
  }

  // Shell scripts & Dockerfiles
  if (name.endsWith('.sh') || name.endsWith('.bash') || name === 'dockerfile' || name.startsWith('dockerfile.')) {
    return [StreamLanguage.define(shell)];
  }

  // Content-based heuristic fallback if filename is unknown
  if (content) {
    const trimmed = content.trim();
    if (trimmed.startsWith('{') || trimmed.startsWith('[')) {
      return [json()];
    }
    if (trimmed.startsWith('services:') || trimmed.startsWith('version:') || trimmed.includes('\nservices:')) {
      return [yaml()];
    }
  }

  return [];
}

// Dokidoki custom theme matching slate-950 palette
const dokidokiEditorTheme = EditorView.theme({
  '&': {
    backgroundColor: '#020617', // slate-950
    color: '#e2e8f0', // slate-200
    fontSize: '12px',
    fontFamily: 'ui-monospace, SFMono-Regular, Menlo, Monaco, Consolas, "Liberation Mono", "Courier New", monospace',
  },
  '.cm-content': {
    padding: '8px 0',
    caretColor: '#f43f5e', // rose-500
  },
  '.cm-cursor': {
    borderLeftColor: '#f43f5e',
    borderLeftWidth: '2px',
  },
  '.cm-gutters': {
    backgroundColor: '#020617',
    borderRight: '1px solid #1e293b', // slate-800
    color: '#64748b', // slate-500
  },
  '.cm-lineNumbers .cm-gutterElement': {
    padding: '0 8px 0 4px',
    minWidth: '28px',
  },
  '.cm-activeLine': {
    backgroundColor: 'rgba(30, 41, 59, 0.35)', // slate-800/35
  },
  '.cm-activeLineGutter': {
    backgroundColor: 'rgba(30, 41, 59, 0.5)',
    color: '#94a3b8', // slate-400
  },
  '.cm-selectionBackground, ::selection': {
    backgroundColor: 'rgba(51, 65, 85, 0.6) !important', // slate-700/60
  },
  '.cm-scroller': {
    lineHeight: '1.6',
    fontFamily: 'inherit',
  },
  '&.cm-focused': {
    outline: 'none',
  },
});

export const CodeEditor: React.FC<CodeEditorProps> = ({
  value,
  onChange,
  filename,
  readOnly = false,
  height,
  minHeight = '120px',
  maxHeight = '480px',
  className = '',
  autoFocus = false,
  placeholder,
}) => {
  const extensions = useMemo(() => {
    const exts: Extension[] = [
      oneDark,
      dokidokiEditorTheme,
      EditorView.lineWrapping,
      indentUnit.of('  '),
    ];

    const lang = getLanguageExtension(filename, value);
    exts.push(...lang);

    return exts;
  }, [filename, value]);

  return (
    <div className={`overflow-hidden rounded-md border border-slate-800 bg-slate-950 text-xs font-mono ${className}`}>
      <CodeMirror
        value={value}
        onChange={onChange}
        readOnly={readOnly}
        editable={!readOnly}
        height={height}
        minHeight={minHeight}
        maxHeight={maxHeight}
        autoFocus={autoFocus}
        placeholder={placeholder}
        extensions={extensions}
        basicSetup={{
          lineNumbers: true,
          highlightActiveLineGutter: !readOnly,
          highlightSpecialChars: true,
          history: !readOnly,
          foldGutter: true,
          drawSelection: true,
          dropCursor: !readOnly,
          allowMultipleSelections: true,
          indentOnInput: !readOnly,
          syntaxHighlighting: true,
          bracketMatching: true,
          closeBrackets: !readOnly,
          autocompletion: !readOnly,
          rectangularSelection: true,
          crosshairCursor: true,
          highlightActiveLine: !readOnly,
          highlightSelectionMatches: true,
          closeBracketsKeymap: !readOnly,
          defaultKeymap: true,
          searchKeymap: true,
          historyKeymap: !readOnly,
          foldKeymap: true,
          completionKeymap: !readOnly,
          lintKeymap: !readOnly,
        }}
      />
    </div>
  );
};
