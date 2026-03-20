<script lang="ts">
  import { auth as authApi, ApiError } from '../lib/api';
  import { setStoredOperator } from '../lib/auth';
  import { currentUser, addToast } from '../lib/stores';

  let { onSuccess }: { onSuccess?: () => void } = $props();

  let username = $state('');
  let password = $state('');
  let error = $state('');
  let submitting = $state(false);

  async function handleLogin(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    submitting = true;

    try {
      const res = await authApi.login(username, password);
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
      <h1 class="text-3xl font-bold text-on-surface tracking-tight">Quorum</h1>
      <p class="text-on-surface-variant mt-2">Sign in to the admin console</p>
    </div>

    <form onsubmit={handleLogin} class="bg-surface-container-lowest rounded-xl shadow-ambient-sm p-8 space-y-6">
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
          required
          autocomplete="username"
          class="w-full px-3 py-2 border border-outline-variant/40 rounded-md shadow-ambient-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
        />
      </div>

      <div>
        <label for="password" class="block text-sm font-medium text-on-surface mb-1">Password</label>
        <input
          id="password"
          type="password"
          bind:value={password}
          required
          autocomplete="current-password"
          class="w-full px-3 py-2 border border-outline-variant/40 rounded-md shadow-ambient-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary"
        />
      </div>

      <button
        type="submit"
        disabled={submitting}
        class="w-full py-2 px-4 bg-gradient-to-br from-primary to-primary-container text-white font-medium rounded-md hover:brightness-110 focus:outline-none focus:ring-2 focus:ring-offset-2 focus:ring-primary disabled:opacity-50 disabled:cursor-not-allowed transition-all"
      >
        {submitting ? 'Signing in...' : 'Sign in'}
      </button>
    </form>
  </div>
</div>
