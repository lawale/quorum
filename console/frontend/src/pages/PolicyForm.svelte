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
  let dynamicAuthorizationUrl = $state('');
  let dynamicAuthorizationSecret = $state('');
  let displayTemplateJson = $state('');
  let displayTemplateError = $state('');
  let isLoading = $state(!!id);
  let submitting = $state(false);
  let error = $state('');
  let isEdit = $derived(!!id);
  let validationErrors: Record<string, string> = $state({});

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
      dynamicAuthorizationUrl = p.dynamic_authorization_url || '';
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

  function validate(): boolean {
    validationErrors = {};

    if (!name.trim() || name.trim().length > 100) {
      validationErrors.name = 'Name is required (max 100 characters)';
    }

    if (!requestType.trim()) {
      validationErrors.requestType = 'Request type is required';
    } else if (requestType.trim().length > 100) {
      validationErrors.requestType = 'Request type must be at most 100 characters';
    } else if (!/^[a-zA-Z0-9_.\-]+$/.test(requestType.trim())) {
      validationErrors.requestType = 'Request type can only contain letters, numbers, underscores, dots, and hyphens';
    }

    for (let i = 0; i < stages.length; i++) {
      if (!stages[i].required_approvals || stages[i].required_approvals < 1) {
        validationErrors[`stage_${i}_approvals`] = 'Required approvals must be at least 1';
      }
    }

    if (identityFields.trim()) {
      const fields = identityFields.split(',').map((s) => s.trim()).filter(Boolean);
      for (let i = 0; i < fields.length; i++) {
        if (!fields[i]) {
          validationErrors.identityFields = 'Identity fields must not contain empty values';
          break;
        }
      }
    }

    if (dynamicAuthorizationUrl.trim()) {
      try {
        new URL(dynamicAuthorizationUrl.trim());
      } catch {
        validationErrors.dynamicAuthorizationUrl = 'Must be a valid URL';
      }
    }

    if (autoExpireDuration.trim()) {
      if (!/^(\d+h)?(\d+m)?(\d+s)?$/.test(autoExpireDuration.trim()) || autoExpireDuration.trim() === '') {
        validationErrors.autoExpireDuration = 'Must be a valid Go duration (e.g. 24h, 30m, 1h30m)';
      }
    }

    return Object.keys(validationErrors).length === 0;
  }

  function clearFieldError(field: string) {
    validationErrors = { ...validationErrors };
    delete validationErrors[field];
  }

  async function handleSubmit(e: SubmitEvent) {
    e.preventDefault();
    error = '';

    if (!validate()) return;

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
    if (dynamicAuthorizationUrl) payload.dynamic_authorization_url = dynamicAuthorizationUrl;
    if (dynamicAuthorizationSecret) payload.dynamic_authorization_secret = dynamicAuthorizationSecret;
    if (displayTemplateJson.trim()) {
      try {
        payload.display_template = JSON.parse(displayTemplateJson);
        displayTemplateError = '';
      } catch {
        displayTemplateError = 'Invalid JSON';
        submitting = false;
        return;
      }
    } else if (isEdit) {
      payload.display_template = null;
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
    <a href="#/policies" class="text-sm text-on-surface-variant hover:text-on-surface">&larr; Back to policies</a>
    <h1 class="text-2xl font-bold text-on-surface mt-2">{isEdit ? 'Edit Policy' : 'Create Policy'}</h1>
  </div>

  {#if isLoading}
    <LoadingSpinner />
  {:else}
    <form onsubmit={handleSubmit} class="bg-surface-container-lowest shadow-ambient-sm rounded-xl p-6 space-y-6 max-w-3xl">
      {#if error}
        <div class="bg-status-rejected-bg border-status-rejected-text/20 text-status-rejected-text px-4 py-3 rounded text-sm">{error}</div>
      {/if}

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label for="name" class="block text-sm font-medium text-on-surface mb-1">Name</label>
          <input id="name" type="text" bind:value={name} oninput={() => clearFieldError('name')} required class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.name ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
          {#if validationErrors.name}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.name}</p>
          {/if}
        </div>
        <div>
          <label for="requestType" class="block text-sm font-medium text-on-surface mb-1">Request Type</label>
          <input id="requestType" type="text" bind:value={requestType} oninput={() => clearFieldError('requestType')} required disabled={isEdit} class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary disabled:bg-surface-container {validationErrors.requestType ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
          {#if validationErrors.requestType}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.requestType}</p>
          {/if}
        </div>
      </div>

      <!-- Stages -->
      <div>
        <div class="flex items-center justify-between mb-3">
          <label class="block text-sm font-medium text-on-surface">Approval Stages</label>
          <button type="button" onclick={addStage} class="text-sm text-primary-container hover:text-primary">+ Add Stage</button>
        </div>
        <div class="space-y-3">
          {#each stages as stage, i}
            <div class="rounded-lg p-4 bg-surface-container-low">
              <div class="flex items-center justify-between mb-3">
                <span class="text-sm font-medium text-on-surface">Stage {i + 1}</span>
                {#if stages.length > 1}
                  <button type="button" onclick={() => removeStage(i)} class="text-xs text-status-rejected-text hover:text-red-800">Remove</button>
                {/if}
              </div>
              <div class="grid grid-cols-3 gap-3">
                <div>
                  <label class="block text-xs text-on-surface-variant mb-1">Name</label>
                  <input type="text" bind:value={stages[i].name} class="w-full px-2 py-1.5 text-sm border border-outline-variant/40 rounded-md" />
                </div>
                <div>
                  <label class="block text-xs text-on-surface-variant mb-1">Required Approvals</label>
                  <input type="number" min="1" bind:value={stages[i].required_approvals} oninput={() => clearFieldError(`stage_${i}_approvals`)} class="w-full px-2 py-1.5 text-sm border rounded-md {validationErrors[`stage_${i}_approvals`] ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
                  {#if validationErrors[`stage_${i}_approvals`]}
                    <p class="mt-1 text-xs text-status-rejected-text">{validationErrors[`stage_${i}_approvals`]}</p>
                  {/if}
                </div>
                <div>
                  <label class="block text-xs text-on-surface-variant mb-1">Rejection Policy</label>
                  <select bind:value={stages[i].rejection_policy} class="w-full px-2 py-1.5 text-sm border border-outline-variant/40 rounded-md">
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
          <label for="identityFields" class="block text-sm font-medium text-on-surface mb-1">Identity Fields <span class="text-on-surface-variant/60">(comma-separated)</span></label>
          <input id="identityFields" type="text" bind:value={identityFields} oninput={() => clearFieldError('identityFields')} placeholder="account_id, user_id" class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.identityFields ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
          {#if validationErrors.identityFields}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.identityFields}</p>
          {/if}
        </div>
        <div>
          <label for="autoExpire" class="block text-sm font-medium text-on-surface mb-1">Auto Expire Duration</label>
          <input id="autoExpire" type="text" bind:value={autoExpireDuration} oninput={() => clearFieldError('autoExpireDuration')} placeholder="24h, 30m" class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.autoExpireDuration ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
          {#if validationErrors.autoExpireDuration}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.autoExpireDuration}</p>
          {/if}
        </div>
      </div>

      <div class="grid grid-cols-2 gap-4">
        <div>
          <label for="permUrl" class="block text-sm font-medium text-on-surface mb-1">Dynamic Authorization URL <span class="text-on-surface-variant/60">(optional)</span></label>
          <input id="permUrl" type="url" bind:value={dynamicAuthorizationUrl} oninput={() => clearFieldError('dynamicAuthorizationUrl')} placeholder="https://..." class="w-full px-3 py-2 border rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary {validationErrors.dynamicAuthorizationUrl ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
          {#if validationErrors.dynamicAuthorizationUrl}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.dynamicAuthorizationUrl}</p>
          {/if}
        </div>
        <div>
          <label for="permSecret" class="block text-sm font-medium text-on-surface mb-1">Authorization Secret <span class="text-on-surface-variant/60">(optional)</span></label>
          <input id="permSecret" type="password" bind:value={dynamicAuthorizationSecret} placeholder={isEdit ? 'Leave blank to keep unchanged' : 'HMAC signing secret'} class="w-full px-3 py-2 border border-outline-variant/40 rounded-md shadow-sm focus:outline-none focus:ring-2 focus:ring-primary focus:border-primary" />
        </div>
      </div>

      <div>
        <label for="displayTemplate" class="block text-sm font-medium text-on-surface mb-1">Display Template <span class="text-on-surface-variant/60">(optional JSON)</span></label>
        <textarea
          id="displayTemplate"
          bind:value={displayTemplateJson}
          oninput={() => displayTemplateError = ''}
          rows="6"
          placeholder="See docs for template format"
          class="w-full px-3 py-2 border rounded-md shadow-sm font-mono text-sm focus:outline-none focus:ring-2 focus:ring-primary {displayTemplateError ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}"
        ></textarea>
        {#if displayTemplateError}
          <p class="mt-1 text-xs text-status-rejected-text">{displayTemplateError}</p>
        {/if}
        <p class="mt-1 text-xs text-on-surface-variant/60">Maps payload fields to human-readable labels for reviewers. Formatters: currency, date, number, truncate.</p>
      </div>

      <div class="flex items-center gap-3 pt-4 border-t border-outline-variant/15">
        <button type="submit" disabled={submitting} class="px-4 py-2 text-sm font-medium text-white bg-gradient-to-br from-primary to-primary-container rounded-md hover:brightness-110 disabled:opacity-50 transition-all">
          {submitting ? 'Saving...' : isEdit ? 'Update Policy' : 'Create Policy'}
        </button>
        <a href="#/policies" class="px-4 py-2 text-sm text-on-surface-variant hover:text-on-surface">Cancel</a>
      </div>
    </form>
  {/if}
</div>
