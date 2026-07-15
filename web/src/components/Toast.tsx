import { useStore } from '../store';

export default function Toast() {
  const toast = useStore(s => s.toast);
  if (!toast) return null;
  const icons: Record<string, string> = { success: '✓', error: '✗', warning: '⚠', info: 'ⓘ' };
  return (
    <div className={`toast toast-${toast.type}`}>
      <span className="toast-icon">{icons[toast.type] || ''}</span>
      <span>{toast.msg}</span>
    </div>
  );
}