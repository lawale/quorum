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

<div class="min-h-screen bg-gray-50 flex items-center justify-center px-4">
  <div class="max-w-md w-full">
    <div class="text-center mb-8">
      <h1 class="text-3xl font-bold text-gray-900">Quorum Setup</h1>
      <p class="text-gray-500 mt-2">Create your admin account to get started</p>
    </div>

    <form onsubmit={handleSetup} class="bg-white rounded-lg shadow-sm border border-gray-200 p-8 space-y-6">
      {#if error}
        <div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded text-sm">
          {error}
        </div>
      {/if}

      <div>
        <label for="username" class="block text-sm font-medium text-gray-700 mb-1">Username</label>
        <input
          id="username"
          type="text"
          bind:value={username}
          oninput={() => clearFieldError('username')}
          required
          autocomplete="username"
          class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 {validationErrors.username ? 'border-red-300' : 'border-gray-300'}"
        />
        {#if validationErrors.username}
          <p class="mt-1 text-xs text-red-600">{validationErrors.username}</p>
        {/if}
      </div>

      <div>
        <label for="display-name" class="block text-sm font-medium text-gray-700 mb-1">Display Name <span class="text-gray-400">(optional)</span></label>
        <input
          id="display-name"
          type="text"
          bind:value={displayName}
          oninput={() => clearFieldError('displayName')}
          autocomplete="name"
          class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 {validationErrors.displayName ? 'border-red-300' : 'border-gray-300'}"
        />
        {#if validationErrors.displayName}
          <p class="mt-1 text-xs text-red-600">{validationErrors.displayName}</p>
        {/if}
      </div>

      <div>
        <label for="password" class="block text-sm font-medium text-gray-700 mb-1">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          oninput={() => clearFieldError('password')}
          required
          autocomplete="new-password"
          class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 {validationErrors.password ? 'border-red-300' : 'border-gray-300'}"
        />
        {#if validationErrors.password}
          <p class="mt-1 text-xs text-red-600">{validationErrors.password}</p>
        {/if}
      </div>

      <div>
        <label for="confirm-password" class="block text-sm font-medium text-gray-700 mb-1">Confirm Password</label>
        <input
          id="confirm-password"
          type="password"
          bind:value={confirmPassword}
          oninput={() => clearFieldError('confirmPassword')}
          required
          autocomplete="new-password"
          class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 {validationErrors.confirmPassword ? 'border-red-300' : 'border-gray-300'}"
        />
        {#if validationErrors.confirmPassword}
          <p class="mt-1 text-xs text-red-600">{validationErrors.confirmPassword}</p>
        {/if}
      </div>

      <button
        type="submit"
        disabled={submitting}
        class="w-full py-2 px-4 bg-indigo-600 text-white font-medium rounded-md hover:bg-indigo-700 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-indigo-500 disabled:opacity-50 disabled:cursor-not-allowed transition-colors"
      >
        {submitting ? 'Creating account...' : 'Create Admin Account'}
      </button>
    </form>
  </div>
</div>
