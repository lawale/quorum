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

  async function handleSetup(e: SubmitEvent) {
    e.preventDefault();
    error = '';

    if (password !== confirmPassword) {
      error = 'Passwords do not match';
      return;
    }

    if (password.length < 6) {
      error = 'Password must be at least 6 characters';
      return;
    }

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
          required
          autocomplete="username"
          class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
        />
      </div>

      <div>
        <label for="display-name" class="block text-sm font-medium text-gray-700 mb-1">Display Name <span class="text-gray-400">(optional)</span></label>
        <input
          id="display-name"
          type="text"
          bind:value={displayName}
          autocomplete="name"
          class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
        />
      </div>

      <div>
        <label for="password" class="block text-sm font-medium text-gray-700 mb-1">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          required
          autocomplete="new-password"
          class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
        />
      </div>

      <div>
        <label for="confirm-password" class="block text-sm font-medium text-gray-700 mb-1">Confirm Password</label>
        <input
          id="confirm-password"
          type="password"
          bind:value={confirmPassword}
          required
          autocomplete="new-password"
          class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500"
        />
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
