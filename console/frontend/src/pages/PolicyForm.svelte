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

  let identityFieldsList = $derived(
    identityFields.split(',').map(s => s.trim()).filter(Boolean)
  );
  let newIdentityField = $state('');

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
      autoExpireDuration = p.auto_expire_duration ? String(p.auto_expire_duration) : '';
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

  function addIdentityField() {
    const field = newIdentityField.trim();
    if (!field) return;
    const current = identityFields.split(',').map(s => s.trim()).filter(Boolean);
    if (current.includes(field)) return;
    current.push(field);
    identityFields = current.join(', ');
    newIdentityField = '';
  }

  function removeIdentityField(field: string) {
    const current = identityFields.split(',').map(s => s.trim()).filter(Boolean);
    identityFields = current.filter(f => f !== field).join(', ');
  }

  function handleIdentityFieldKeydown(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      addIdentityField();
    }
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

{#if isLoading}
  <LoadingSpinner />
{:else}
  <form onsubmit={handleSubmit} class="max-w-5xl mx-auto">
    <!-- Breadcrumb -->
    <nav class="text-xs text-on-surface-variant mb-4">
      <a href="#/policies" class="hover:text-on-surface transition-colors">Policies</a>
      <span class="mx-1.5">/</span>
      <span class="text-on-surface">{name || 'New Policy'}</span>
    </nav>

    <!-- Page title -->
    <div class="flex items-center gap-3 mb-10">
      <h1 class="text-3xl font-bold text-on-surface tracking-tight">{name || 'New Policy'}</h1>
      {#if isEdit}
        <span class="px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-widest bg-status-approved-bg text-status-approved-text">Active</span>
      {:else}
        <span class="px-2.5 py-0.5 rounded-full text-[10px] font-bold uppercase tracking-widest bg-surface-container text-on-surface-variant">Draft</span>
      {/if}
    </div>

    {#if error}
      <div class="bg-status-rejected-bg border border-status-rejected-text/20 text-status-rejected-text px-4 py-3 rounded-lg text-sm mb-8">{error}</div>
    {/if}

    <!-- A. Basic Info -->
    <section class="grid grid-cols-1 lg:grid-cols-3 gap-8 py-10 border-b border-outline-variant/15">
      <div>
        <h2 class="text-lg font-bold text-on-surface mb-2">Basic Info</h2>
        <p class="text-sm text-on-surface-variant">Fundamental identification and connectivity parameters for this policy definition.</p>
      </div>
      <div class="lg:col-span-2">
        <div class="grid grid-cols-2 gap-4 mb-4">
          <div>
            <label for="name" class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Policy Name</label>
            <input id="name" type="text" bind:value={name} oninput={() => clearFieldError('name')} required class="w-full px-3 py-2.5 text-sm border rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary {validationErrors.name ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
            {#if validationErrors.name}
              <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.name}</p>
            {/if}
          </div>
          <div>
            <label for="requestType" class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Request Type</label>
            <input id="requestType" type="text" bind:value={requestType} oninput={() => clearFieldError('requestType')} required disabled={isEdit} class="w-full px-3 py-2.5 text-sm border rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary disabled:bg-surface-container disabled:cursor-not-allowed {validationErrors.requestType ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
            {#if validationErrors.requestType}
              <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.requestType}</p>
            {/if}
          </div>
        </div>
        <div>
          <label for="permUrl" class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Dynamic Auth URL</label>
          <input id="permUrl" type="url" bind:value={dynamicAuthorizationUrl} oninput={() => clearFieldError('dynamicAuthorizationUrl')} placeholder="https://..." class="w-full px-3 py-2.5 text-sm border rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary {validationErrors.dynamicAuthorizationUrl ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
          {#if validationErrors.dynamicAuthorizationUrl}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.dynamicAuthorizationUrl}</p>
          {/if}
        </div>
      </div>
    </section>

    <!-- B. Expiry & Identity -->
    <section class="grid grid-cols-1 lg:grid-cols-3 gap-8 py-10 border-b border-outline-variant/15">
      <div>
        <h2 class="text-lg font-bold text-on-surface mb-2">Expiry & Identity</h2>
        <p class="text-sm text-on-surface-variant">Determine how long a request stays valid and what metadata is required for identification.</p>
      </div>
      <div class="lg:col-span-2 space-y-6">
        <div>
          <label for="autoExpire" class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Auto-expire Duration</label>
          <div class="flex items-center gap-3">
            <input id="autoExpire" type="text" bind:value={autoExpireDuration} oninput={() => clearFieldError('autoExpireDuration')} placeholder="24h, 30m, 1h30m" class="w-full max-w-xs px-3 py-2.5 text-sm border rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary {validationErrors.autoExpireDuration ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
            {#if autoExpireDuration.trim()}
              <span class="px-3 py-1.5 rounded-full text-xs font-bold bg-primary-container/15 text-primary border border-primary/20">{autoExpireDuration}</span>
            {/if}
          </div>
          {#if validationErrors.autoExpireDuration}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.autoExpireDuration}</p>
          {/if}
        </div>

        <div>
          <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Identity Fields</label>
          <div class="flex flex-wrap items-center gap-2 mb-3">
            {#each identityFieldsList as field}
              <span class="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-full text-xs font-medium bg-surface-container text-on-surface border border-outline-variant/30">
                {field}
                <button type="button" onclick={() => removeIdentityField(field)} class="text-on-surface-variant hover:text-status-rejected-text transition-colors" aria-label="Remove {field}">
                  <svg class="w-3 h-3" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2.5"><path d="M6 18L18 6M6 6l12 12" /></svg>
                </button>
              </span>
            {/each}
          </div>
          <div class="flex items-center gap-2">
            <input type="text" bind:value={newIdentityField} onkeydown={handleIdentityFieldKeydown} placeholder="e.g. account_id" class="px-3 py-2 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary" />
            <button type="button" onclick={addIdentityField} class="text-sm font-medium text-primary hover:text-primary-container transition-colors">+ Add Field</button>
          </div>
          {#if validationErrors.identityFields}
            <p class="mt-1 text-xs text-status-rejected-text">{validationErrors.identityFields}</p>
          {/if}
        </div>
      </div>
    </section>

    <!-- C. Approval Stages -->
    <section class="grid grid-cols-1 lg:grid-cols-3 gap-8 py-10 border-b border-outline-variant/15">
      <div>
        <h2 class="text-lg font-bold text-on-surface mb-2">Approval Stages</h2>
        <p class="text-sm text-on-surface-variant">Sequential logic gates required for policy finalization.</p>
      </div>
      <div class="lg:col-span-2">
        <div class="flex justify-end mb-4">
          <button type="button" onclick={addStage} class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-4 py-2 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 transition-all">
            + New Stage
          </button>
        </div>

        <div class="space-y-4">
          {#each stages as stage, i}
            <div class="bg-surface-container-lowest rounded-xl shadow-ambient-sm p-6 relative">
              <!-- Delete button -->
              {#if stages.length > 1}
                <button type="button" onclick={() => removeStage(i)} class="absolute top-4 right-4 text-on-surface-variant/50 hover:text-status-rejected-text transition-colors" aria-label="Remove stage {i + 1}">
                  <svg class="w-4 h-4" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="2"><path d="M19 7l-.867 12.142A2 2 0 0116.138 21H7.862a2 2 0 01-1.995-1.858L5 7m5 4v6m4-6v6m1-10V4a1 1 0 00-1-1h-4a1 1 0 00-1 1v3M4 7h16" /></svg>
                </button>
              {/if}

              <!-- Stage header -->
              <div class="flex items-center gap-3 mb-5">
                <span class="w-8 h-8 rounded-lg bg-primary-container text-white flex items-center justify-center text-sm font-bold">{i + 1}</span>
                <div>
                  <h3 class="font-bold text-on-surface">{stage.name || 'Untitled Stage'}</h3>
                  <p class="text-[10px] font-mono text-on-surface-variant">STAGE IDENTIFIER: STG_{i + 1}</p>
                </div>
              </div>

              <!-- Stage fields -->
              <div class="grid grid-cols-3 gap-4">
                <div>
                  <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Stage Name</label>
                  <input type="text" bind:value={stages[i].name} class="w-full px-3 py-2 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary" />
                </div>
                <div>
                  <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Required Approvals</label>
                  <input type="number" min="1" bind:value={stages[i].required_approvals} oninput={() => clearFieldError(`stage_${i}_approvals`)} class="w-full px-3 py-2 text-sm border rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary {validationErrors[`stage_${i}_approvals`] ? 'border-status-rejected-text/30' : 'border-outline-variant/40'}" />
                  {#if validationErrors[`stage_${i}_approvals`]}
                    <p class="mt-1 text-xs text-status-rejected-text">{validationErrors[`stage_${i}_approvals`]}</p>
                  {/if}
                </div>
                <div>
                  <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Rejection Policy</label>
                  <select bind:value={stages[i].rejection_policy} class="w-full px-3 py-2 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary">
                    <option value="any">Any rejection</option>
                    <option value="threshold">Threshold</option>
                  </select>
                </div>
                <div>
                  <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Auth Mode</label>
                  <input type="text" bind:value={stages[i].authorization_mode} placeholder="e.g. role_based" class="w-full px-3 py-2 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary" />
                </div>
                <div>
                  <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Max Checkers</label>
                  <input type="number" min="1" bind:value={stages[i].max_checkers} placeholder="--" class="w-full px-3 py-2 text-sm border border-outline-variant/40 rounded-lg bg-surface-container-lowest focus:outline-none focus:ring-2 focus:ring-primary" />
                </div>
                <div>
                  <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Checker Roles</label>
                  <div class="flex flex-wrap gap-1.5">
                    {#if stage.allowed_checker_roles?.length}
                      {#each stage.allowed_checker_roles as role}
                        <span class="px-2 py-0.5 rounded-full text-[10px] font-medium bg-surface-container text-on-surface-variant border border-outline-variant/30">{role}</span>
                      {/each}
                    {:else}
                      <span class="text-xs text-on-surface-variant/50 italic">Any role</span>
                    {/if}
                  </div>
                </div>
              </div>

              {#if stage.allowed_permissions?.length}
                <div class="mt-4 pt-4 border-t border-outline-variant/10">
                  <label class="block text-[10px] font-bold uppercase tracking-widest text-on-surface-variant mb-2">Permissions</label>
                  <div class="flex flex-wrap gap-1.5">
                    {#each stage.allowed_permissions as perm}
                      <span class="px-2 py-0.5 rounded-full text-[10px] font-medium bg-surface-container text-on-surface-variant border border-outline-variant/30">{perm}</span>
                    {/each}
                  </div>
                </div>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    </section>

    <!-- D. Display Template -->
    <section class="grid grid-cols-1 lg:grid-cols-3 gap-8 py-10 border-b border-outline-variant/15">
      <div>
        <h2 class="text-lg font-bold text-on-surface mb-2">Display Template</h2>
        <p class="text-sm text-on-surface-variant">Customize how the request appears to approvers in the mobile and web interface using JSON logic.</p>
      </div>
      <div class="lg:col-span-2">
        <div class="bg-on-surface rounded-xl p-1 overflow-hidden">
          <div class="flex gap-1.5 px-4 py-2">
            <div class="w-3 h-3 rounded-full bg-red-500"></div>
            <div class="w-3 h-3 rounded-full bg-yellow-500"></div>
            <div class="w-3 h-3 rounded-full bg-green-500"></div>
          </div>
          <textarea
            id="displayTemplate"
            bind:value={displayTemplateJson}
            oninput={() => displayTemplateError = ''}
            placeholder='&#123; "title": "...", "fields": [...] &#125;'
            class="w-full bg-transparent text-emerald-400 font-mono text-sm p-4 focus:outline-none min-h-[200px] resize-y"
          ></textarea>
        </div>
        {#if displayTemplateError}
          <p class="mt-2 text-xs text-status-rejected-text">{displayTemplateError}</p>
        {/if}
        <p class="mt-2 text-xs text-on-surface-variant/60">Maps payload fields to human-readable labels for reviewers. Formatters: currency, date, number, truncate.</p>
      </div>
    </section>

    <!-- Footer -->
    <div class="flex items-center justify-between py-6 mt-6 border-t border-outline-variant/15">
      <p class="text-xs text-on-surface-variant">Last saved: {isEdit ? 'recently' : 'not yet'}</p>
      <div class="flex gap-3">
        <a href="#/policies" class="px-4 py-2.5 text-sm font-medium text-on-surface-variant hover:text-on-surface transition-colors">Cancel</a>
        <button type="submit" disabled={submitting} class="bg-gradient-to-br from-primary to-primary-container text-on-primary px-6 py-2.5 rounded-md font-semibold text-sm shadow-lg shadow-primary/20 hover:brightness-110 disabled:opacity-50 transition-all">
          {submitting ? 'Saving...' : 'Save Changes'}
        </button>
      </div>
    </div>
  </form>
{/if}
