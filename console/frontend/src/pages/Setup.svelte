<script lang="ts">
  import { auth as authApi, ApiError } from '../lib/api';
  import { setStoredOperator } from '../lib/auth';
  import { currentUser } from '../lib/stores';

  let { onSuccess }: { onSuccess?: () => void } = $props();

  let username = $state('');
  let password = $state('');
  let confirmPassword = $state('');
  let displayName = $state('');
  let error = $state('');
  let submitting = $state(false);
  let validationErrors: Record<string, string> = $state({});

  function validate(): boolean {
    validationErrors = {};

    if (username.trim().length < 3) {
      validationErrors.username = 'Username must be at least 3 characters';
    } else if (!/^[a-zA-Z0-9_]+$/.test(username.trim())) {
      validationErrors.username = 'Username can only contain letters, numbers, and underscores';
    }

    if (password.length < 8) {
      validationErrors.password = 'Password must be at least 8 characters';
    } else if (!/[A-Z]/.test(password)) {
      validationErrors.password = 'Password must contain at least one uppercase letter';
    } else if (!/[0-9]/.test(password)) {
      validationErrors.password = 'Password must contain at least one digit';
    }

    if (password !== confirmPassword) {
      validationErrors.confirmPassword = 'Passwords do not match';
    }

    if (displayName.trim().length > 0 && displayName.trim().length < 1) {
      validationErrors.displayName = 'Display name must not be empty';
    }

    return Object.keys(validationErrors).length === 0;
  }

  function clearFieldError(field: string) {
    validationErrors = { ...validationErrors };
    delete validationErrors[field];
  }

  async function handleSetup(e: SubmitEvent) {
    e.preventDefault();
    error = '';

    if (!validate()) return;

    submitting = true;

    try {
      const res = await authApi.setup(username, password, displayName || username);
      setStoredOperator(res.operator);
      currentUser.set(res.operator);
      onSuccess ? onSuccess() : (window.location.hash = '#/');
    } catch (err) {
      if (err instanceof ApiError) {
        error = err.message;
      } else {
        error = 'An unexpected error occurred';
      }
    } finally {
      submitting = false;
    }
  }
</script>

<div class="min-h-screen bg-surface flex items-center justify-center px-4">
  <div class="max-w-md w-full">
    <div class="text-center mb-8">
      <h1 class="text-3xl font-bold text-on-surface tracking-tight">Quorum Setup</h1>
      <p class="text-on-surface-variant mt-2">Create your admin account to get started</p>
    </div>

    <form onsubmit={handleSetup} class="bg-surface-container-lowest rounded-xl shadow-ambient-sm p-8 space-y-6">
      {#if error}
        <div class="bg-status-rejected-bg border border-status-rejected-text/20 text-status-rejected-text px-4 py-3 rounded text-sm">
          {error}
        </div>
      {/if}

      <div>
        <label for="username" class="block text-sm font-medium text-on-surface mb-1">Username</label>
        <input
          id="username"
          type="text"
          bind:value={username}
          oninput={() => clearFieldError('username')}
          required
          autocomplete="username"
          class="w-full px-3 py-2 border rounded-md shadow-ambient-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.username ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
        />
        {#if validationErrors.username}
          <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.username}</p>
        {/if}
      </div>

      <div>
        <label for="display-name" class="block text-sm font-medium text-on-surface mb-1">Display Name <span class="text-on-surface-variant/60">(optional)</span></label>
        <input
          id="display-name"
          type="text"
          bind:value={displayName}
          oninput={() => clearFieldError('displayName')}
          autocomplete="name"
          class="w-full px-3 py-2 border rounded-md shadow-ambient-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.displayName ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
        />
        {#if validationErrors.displayName}
          <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.displayName}</p>
        {/if}
      </div>

      <div>
        <label for="password" class="block text-sm font-medium text-on-surface mb-1">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          oninput={() => clearFieldError('password')}
          required
          autocomplete="new-password"
          class="w-full px-3 py-2 border rounded-md shadow-ambient-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.password ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
        />
        {#if validationErrors.password}
          <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.password}</p>
        {/if}
      </div>

      <div>
        <label for="confirm-password" class="block text-sm font-medium text-on-surface mb-1">Confirm Password</label>
        <input
          id="confirm-password"
          type="password"
          bind:value={confirmPassword}
          oninput={() => clearFieldError('confirmPassword')}
          required
          autocomplete="new-password"
          class="w-full px-3 py-2 border rounded-md shadow-ambient-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.confirmPassword ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
        />
        {#if validationErrors.confirmPassword}
          <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.confirmPassword}</p>
        {/if}
      </div>

      <button
        type="submit"
        disabled={submitting}
        class="w-full py-2 px-4 bg-gradient-to-br from-primary to-primary-container text-white font-medium rounded-md hover:brightness-110 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50 disabled:cursor-not-allowed transition-all"
      >
        {submitting ? 'Creating account...' : 'Create Admin Account'}
      </button>
    </form>
  </div>
</div>
