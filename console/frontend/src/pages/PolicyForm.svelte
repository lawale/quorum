<script lang="ts">
  import { policies as policiesApi, ApiError } from '../lib/api';
  import { addToast } from '../lib/stores';
  import LoadingSpinner from '../components/LoadingSpinner.svelte';
  import type { Policy, ApprovalStage } from '../lib/types';

  let { id }: { id?: string } = $props();

  let name = $state('');
  let requestType = $state('');
  let stages: ApprovalStage[] = $state([{ index: 0, name: 'Default', required_approvals: 1, rejection_policy: 'any' }]);
  let identityFields = $state('');
  let autoExpireDuration = $state('');
  let permissionCheckUrl = $state('');
  let displayTemplateJson = $state('');
  let displayTemplateError = $state('');
  let isLoading = $state(!!id);
  let submitting = $state(false);
  let error = $state('');
  let isEdit = $derived(!!id);

  $effect(() => {
    if (id) loadPolicy(id);
  });

  async function loadPolicy(policyId: string) {
    isLoading = true;
    try {
      const p = await policiesApi.get(policyId);
      name = p.name;
      requestType = p.request_type;
      stages = p.stages;
      identityFields = (p.identity_fields || []).join(', ');
      autoExpireDuration = p.auto_expire_duration || '';
      permissionCheckUrl = p.permission_check_url || '';
      displayTemplateJson = p.display_template ? JSON.stringify(p.display_template, null, 2) : '';
    } catch { addToast('Failed to load policy', 'error'); window.location.hash = '#/policies'; }
    finally { isLoading = false; }
  }

  function addStage() {
    stages = [...stages, { index: stages.length, name: `Stage ${stages.length + 1}`, required_approvals: 1, rejection_policy: 'any' }];
  }

  function removeStage(idx: number) {
    stages = stages.filter((_, i) => i !== idx).map((s, i) => ({ ...s, index: i }));
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = '';
    submitting = true;

    const payload: Partial<Policy> = {
      name,
      request_type: requestType,
      stages: stages.map((s, i) => ({ ...s, index: i })),
    };
    if (identityFields.trim()) {
      payload.identity_fields = identityFields.split(',').map((s) => s.trim()).filter(Boolean);
    }
    if (autoExpireDuration) payload.auto_expire_duration = autoExpireDuration;
    if (permissionCheckUrl) payload.permission_check_url = permissionCheckUrl;
    if (displayTemplateJson.trim()) {
      try {
        payload.display_template = JSON.parse(displayTemplateJson);
        displayTemplateError = '';
      } catch {
        displayTemplateError = 'Invalid JSON';
        submitting = false;
        return;
      }
    }

    try {
      if (isEdit && id) {
        await policiesApi.update(id, payload);
        addToast('Policy updated', 'success');
      } else {
        await policiesApi.create(payload);
        addToast('Policy created', 'success');
      }
      window.location.hash = '#/policies';
    } catch (err) {
      error = err instanceof ApiError ? err.message : 'Failed to save policy';
    } finally {
      submitting = false;
    }
  }
</script>

<div>
  <div class="mb-6">
    <a href="#/policies" class="text-sm text-gray-500 hover:text-gray-700">&larr; Back to policies</a>
    <h1 class="text-2xl font-bold text-gray-900 mt-2">{isEdit ? 'Edit Policy' : 'Create Policy'}</h1>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else}
    <form onsubmit={handleSubmit} class="bg-white shadow-sm rounded-lg border border-gray-200 p-6 space-y-6 max-w-3xl">
      {#if error}
        <div class="bg-red-50 border border-red-200 text-red-700 px-4 py-3 rounded text-sm">{error}</div>
      {/if}

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label for="name" class="block text-sm font-medium text-gray-700 mb-1">Name</label>
          <input id="name" type="text" bind:value={name} required class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500" />
        </div>
        <div>
          <label for="requestType" class="block text-sm font-medium text-gray-700 mb-1">Request Type</label>
          <input id="requestType" type="text" bind:value={requestType} required disabled={isEdit} class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500 disabled:bg-gray-100" />
        </div>
      </div>

      <!-- Stages -->
      <div>
        <div class="flex items-center justify-between mb-3">
          <label class="block text-sm font-medium text-gray-700">Approval Stages</label>
          <button type="button" onclick={addStage} class="text-sm text-indigo-600 hover:text-indigo-800">+ Add Stage</button>
        </div>
        <div class="space-y-3">
          {#each stages as stage, i}
            <div class="border border-gray-200 rounded-md p-4 bg-gray-50">
              <div class="flex items-center justify-between mb-3">
                <span class="text-sm font-medium text-gray-700">Stage {i + 1}</span>
                {#if stages.length > 1}
                  <button type="button" onclick={() => removeStage(i)} class="text-xs text-red-600 hover:text-red-800">Remove</button>
                {/if}
              </div>
              <div class="grid grid-cols-3 gap-3">
                <div>
                  <label class="block text-xs text-gray-500 mb-1">Name</label>
                  <input type="text" bind:value={stages[i].name} class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md" />
                </div>
                <div>
                  <label class="block text-xs text-gray-500 mb-1">Required Approvals</label>
                  <input type="number" min="1" bind:value={stages[i].required_approvals} class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md" />
                </div>
                <div>
                  <label class="block text-xs text-gray-500 mb-1">Rejection Policy</label>
                  <select bind:value={stages[i].rejection_policy} class="w-full px-2 py-1.5 text-sm border border-gray-300 rounded-md">
                    <option value="any">Any rejection</option>
                    <option value="threshold">Threshold</option>
                  </select>
                </div>
              </div>
            </div>
          {/each}
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label for="identityFields" class="block text-sm font-medium text-gray-700 mb-1">Identity Fields <span class="text-gray-400">(comma-separated)</span></label>
          <input id="identityFields" type="text" bind:value={identityFields} placeholder="account_id, user_id" class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500" />
        </div>
        <div>
          <label for="autoExpire" class="block text-sm font-medium text-gray-700 mb-1">Auto Expire Duration</label>
          <input id="autoExpire" type="text" bind:value={autoExpireDuration} placeholder="24h, 30m" class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500" />
        </div>
      </div>

      <div>
        <label for="permUrl" class="block text-sm font-medium text-gray-700 mb-1">Permission Check URL <span class="text-gray-400">(optional)</span></label>
        <input id="permUrl" type="url" bind:value={permissionCheckUrl} placeholder="https://..." class="w-full px-3 py-2 border border-gray-300 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 focus:border-indigo-500" />
      </div>

      <div>
        <label for="displayTemplate" class="block text-sm font-medium text-gray-700 mb-1">Display Template <span class="text-gray-400">(optional JSON)</span></label>
        <textarea
          id="displayTemplate"
          bind:value={displayTemplateJson}
          oninput={() => displayTemplateError = ''}
          rows="6"
          placeholder="See docs for template format"
          class="w-full px-3 py-2 border rounded-md shadow-sm font-mono text-sm focus:outline-none focus:ring-2 focus:ring-indigo-500 {displayTemplateError ? 'border-red-300' : 'border-gray-300'}"
        ></textarea>
        {#if displayTemplateError}
          <p class="mt-1 text-xs text-red-600">{displayTemplateError}</p>
        {/if}
        <p class="mt-1 text-xs text-gray-400">Maps payload fields to human-readable labels for reviewers. Formatters: currency, date, number, truncate.</p>
      </div>

      <div class="flex items-center gap-3 pt-4 border-t border-gray-200">
        <button type="submit" disabled={submitting} class="px-4 py-2 text-sm font-medium text-white bg-indigo-600 rounded-md hover:bg-indigo-700 disabled:opacity-50 transition-colors">
          {submitting ? 'Saving...' : isEdit ? 'Update Policy' : 'Create Policy'}
        </button>
        <a href="#/policies" class="px-4 py-2 text-sm text-gray-700 hover:text-gray-900">Cancel</a>
      </div>
    </form>
  {/if}
</div>
