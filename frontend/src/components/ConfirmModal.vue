<template>
  <div v-if="visible" class="modal-overlay" @click.self="handleCancel">
    <div class="modal-content">
      <div class="modal-header">
        <span class="warning-icon">⚠️</span>
        <h3>安全确认</h3>
      </div>
      <div class="modal-body">
        <p class="message">{{ message }}</p>
        <p class="command">{{ command }}</p>
      </div>
      <div class="modal-footer">
        <button class="btn-cancel" @click="handleCancel">取消</button>
        <button class="btn-confirm" @click="handleConfirm">确认执行</button>
      </div>
    </div>
  </div>
</template>

<script setup lang="ts">
import { computed } from 'vue';
import { useRiskStore } from '../stores/risk';

const riskStore = useRiskStore();

const visible = computed(() => riskStore.pendingConfirmations.size > 0);
const firstPending = computed(() => {
  const first = riskStore.pendingConfirmations.values().next().value;
  return first || null;
});
const message = computed(() => firstPending.value?.message || '');
const command = computed(() => firstPending.value?.command || '');

function handleConfirm() {
  if (firstPending.value) {
    riskStore.resolveConfirmation(firstPending.value.sessionId, true);
  }
}

function handleCancel() {
  if (firstPending.value) {
    riskStore.resolveConfirmation(firstPending.value.sessionId, false);
  }
}
</script>

<style scoped>
.modal-overlay {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(0, 0, 0, 0.7);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10000;
}

.modal-content {
  background: #2d2d2d;
  border: 1px solid #444;
  border-radius: 8px;
  max-width: 500px;
  width: 90%;
}

.modal-header {
  padding: 20px;
  border-bottom: 1px solid #444;
  display: flex;
  align-items: center;
  gap: 10px;
}

.warning-icon {
  font-size: 24px;
}

.modal-header h3 {
  margin: 0;
  color: #ffcc00;
}

.modal-body {
  padding: 20px;
}

.message {
  color: #d4d4d4;
  margin-bottom: 10px;
}

.command {
  background: #1e1e1e;
  padding: 10px;
  border-radius: 4px;
  font-family: monospace;
  color: #ff6b6b;
}

.modal-footer {
  padding: 15px 20px;
  border-top: 1px solid #444;
  display: flex;
  justify-content: flex-end;
  gap: 10px;
}

button {
  padding: 8px 16px;
  border: none;
  border-radius: 4px;
  cursor: pointer;
}

.btn-cancel {
  background: #555;
  color: #d4d4d4;
}

.btn-confirm {
  background: #cc0000;
  color: white;
}
</style>
