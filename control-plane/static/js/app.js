/**
 * BanhBaoRing Dashboard JavaScript
 * Utilities and integrations for HTMX + Alpine.js
 */

// ============================================
// Copy to Clipboard
// ============================================

window.copyToClipboard = async (text, successMessage = 'Copied!') => {
  try {
    await navigator.clipboard.writeText(text);
    showToast(successMessage, 'success');
    return true;
  } catch (err) {
    console.error('Failed to copy:', err);
    showToast('Failed to copy to clipboard', 'error');
    return false;
  }
};

// Copy button functionality
document.addEventListener('click', (e) => {
  const copyBtn = e.target.closest('[data-copy]');
  if (copyBtn) {
    const text = copyBtn.dataset.copy;
    copyToClipboard(text);
  }
});

// ============================================
// Toast Notifications
// ============================================

window.showToast = (message, variant = 'info') => {
  const container = document.getElementById('toast-container');
  if (!container) return;

  const icons = {
    success: '✓',
    error: '✕',
    warning: '⚠',
    info: 'ℹ',
  };

  const colors = {
    success: 'bg-emerald-500/90 border-emerald-400/50 text-white',
    error: 'bg-red-500/90 border-red-400/50 text-white',
    warning: 'bg-amber-500/90 border-amber-400/50 text-bao-bg',
    info: 'bg-bao-accent/90 border-bao-accent/50 text-white',
  };

  const toast = document.createElement('div');
  toast.className = `flex items-center gap-3 px-4 py-3 rounded-xl shadow-xl backdrop-blur-lg border min-w-[300px] max-w-md animate-slide-in-right ${colors[variant] || colors.info}`;
  toast.innerHTML = `
    <span class="text-lg">${icons[variant] || icons.info}</span>
    <p class="flex-1 text-sm font-medium">${escapeHtml(message)}</p>
    <button class="p-1 hover:bg-white/10 rounded transition-colors" onclick="this.parentElement.remove()">
      <svg class="w-4 h-4" fill="none" stroke="currentColor" viewBox="0 0 24 24">
        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
      </svg>
    </button>
  `;

  container.appendChild(toast);

  // Auto-remove after 5 seconds
  setTimeout(() => {
    toast.style.opacity = '0';
    toast.style.transform = 'translateX(10px)';
    setTimeout(() => toast.remove(), 200);
  }, 5000);
};

// ============================================
// HTMX Configuration
// ============================================

document.addEventListener('htmx:configRequest', (event) => {
  // Add CSRF token to all requests
  const csrfToken = document.querySelector('meta[name="csrf-token"]')?.content;
  if (csrfToken) {
    event.detail.headers['X-CSRF-Token'] = csrfToken;
  }
});

// Handle HTMX errors
document.addEventListener('htmx:responseError', (event) => {
  const status = event.detail.xhr.status;
  let message = 'Something went wrong. Please try again.';

  if (status === 401) {
    message = 'Your session has expired. Please log in again.';
    setTimeout(() => {
      window.location.href = '/login';
    }, 2000);
  } else if (status === 403) {
    message = 'You do not have permission to perform this action.';
  } else if (status === 404) {
    message = 'The requested resource was not found.';
  } else if (status >= 500) {
    message = 'Server error. Please try again later.';
  }

  showToast(message, 'error');
});

// Handle HTMX success with toast messages from headers
document.addEventListener('htmx:afterRequest', (event) => {
  const toastMessage = event.detail.xhr.getResponseHeader('X-Toast-Message');
  const toastVariant = event.detail.xhr.getResponseHeader('X-Toast-Variant') || 'success';

  if (toastMessage) {
    showToast(toastMessage, toastVariant);
  }
});

// Add loading indicator class to body during requests
document.addEventListener('htmx:beforeRequest', () => {
  document.body.classList.add('htmx-requesting');
});

document.addEventListener('htmx:afterRequest', () => {
  document.body.classList.remove('htmx-requesting');
});

// ============================================
// Modal Helpers
// ============================================

window.openModal = (url) => {
  htmx.ajax('GET', url, {
    target: '#modal-content',
    swap: 'innerHTML',
  }).then(() => {
    window.dispatchEvent(new CustomEvent('modal-open'));
  });
};

window.closeModal = () => {
  window.dispatchEvent(new CustomEvent('modal-close'));
};

// ============================================
// Confirmation Dialog
// ============================================

window.confirmAction = (options) => {
  const { title, message, confirmText = 'Confirm', cancelText = 'Cancel', variant = 'danger', onConfirm } = options;

  const modalContent = document.getElementById('modal-content');
  if (!modalContent) return;

  modalContent.innerHTML = `
    <div class="max-w-sm w-full">
      <div class="flex items-center justify-between p-5 border-b border-bao-border">
        <h3 class="text-lg font-heading font-semibold text-bao-text">${escapeHtml(title)}</h3>
        <button onclick="closeModal()" class="p-1.5 text-bao-muted hover:text-bao-text hover:bg-bao-border/30 rounded-lg transition-colors">
          <svg class="w-5 h-5" fill="none" stroke="currentColor" viewBox="0 0 24 24">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M6 18L18 6M6 6l12 12"/>
          </svg>
        </button>
      </div>
      <div class="p-5">
        <p class="text-bao-muted mb-6">${escapeHtml(message)}</p>
        <div class="flex items-center justify-end gap-3">
          <button onclick="closeModal()" class="px-4 py-2 text-sm font-medium text-bao-muted hover:text-bao-text hover:bg-bao-border/30 rounded-lg transition-colors">
            ${escapeHtml(cancelText)}
          </button>
          <button id="confirm-action-btn" class="px-4 py-2 text-sm font-medium rounded-lg transition-all duration-200 ${variant === 'danger' ? 'bg-red-500 text-white hover:bg-red-600' : 'bg-gradient-to-r from-amber-400 to-rose-500 text-bao-bg hover:shadow-lg'}">
            ${escapeHtml(confirmText)}
          </button>
        </div>
      </div>
    </div>
  `;

  window.dispatchEvent(new CustomEvent('modal-open'));

  document.getElementById('confirm-action-btn').addEventListener('click', () => {
    closeModal();
    if (typeof onConfirm === 'function') {
      onConfirm();
    }
  });
};

// ============================================
// Utility Functions
// ============================================

function escapeHtml(text) {
  const div = document.createElement('div');
  div.textContent = text;
  return div.innerHTML;
}

// Format relative time
window.formatRelativeTime = (date) => {
  const now = new Date();
  const then = new Date(date);
  const diffMs = now - then;
  const diffSec = Math.floor(diffMs / 1000);
  const diffMin = Math.floor(diffSec / 60);
  const diffHr = Math.floor(diffMin / 60);
  const diffDay = Math.floor(diffHr / 24);

  if (diffSec < 60) return 'just now';
  if (diffMin < 60) return `${diffMin}m ago`;
  if (diffHr < 24) return `${diffHr}h ago`;
  if (diffDay < 7) return `${diffDay}d ago`;
  return then.toLocaleDateString();
};

// Format bytes
window.formatBytes = (bytes) => {
  if (bytes === 0) return '0 B';
  const k = 1024;
  const sizes = ['B', 'KB', 'MB', 'GB', 'TB'];
  const i = Math.floor(Math.log(bytes) / Math.log(k));
  return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
};

// Debounce function
window.debounce = (func, wait) => {
  let timeout;
  return (...args) => {
    clearTimeout(timeout);
    timeout = setTimeout(() => func.apply(this, args), wait);
  };
};

// ============================================
// Keyboard Shortcuts
// ============================================

document.addEventListener('keydown', (e) => {
  // Cmd/Ctrl + K for search (if implemented)
  if ((e.metaKey || e.ctrlKey) && e.key === 'k') {
    e.preventDefault();
    const searchInput = document.querySelector('[data-search-input]');
    if (searchInput) {
      searchInput.focus();
    }
  }

  // Escape to close modals
  if (e.key === 'Escape') {
    closeModal();
  }
});

// ============================================
// Initialize
// ============================================

document.addEventListener('DOMContentLoaded', () => {
  console.log('🔔 BanhBaoRing Dashboard initialized');
});

