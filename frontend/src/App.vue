<template>
  <div class="container">
    <h1>Multi-Terminal</h1>
    <div class="toolbar">
      <span>模式: {{ mode }}</span>
      <span>已选: {{ selectedIds.join(', ') || '无' }}</span>
    </div>
    <div class="terminal-grid">
      <div
        v-for="session in sessions"
        :key="session.id"
        :class="['terminal-wrapper', { active: session.id === activeId, selected: clientStore.isSelected(session.id), slow: session.isSlow }]"
        @click="handleClick(session.id)"
        @click.shift="handleShiftClick(session.id)"
      >
        <Terminal :sessionId="session.id" ref="terminals" />
      </div>
    </div>
    <ConfirmModal />
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted } from 'vue';
import { useClientStore } from './stores/client';
import { useRiskStore } from './stores/risk';
import Terminal from './components/Terminal.vue';
import ConfirmModal from './components/ConfirmModal.vue';

const clientStore = useClientStore();
const riskStore = useRiskStore();
const activeId = computed(() => clientStore.activeSessionId);
const selectedIds = computed(() => clientStore.selectedSessionIds);
const mode = computed(() => clientStore.mode);

interface SessionInfo {
  id: number;
  isSlow: boolean;
}

const sessions = ref<SessionInfo[]>([
  { id: 1, isSlow: false },
  { id: 2, isSlow: false },
  { id: 3, isSlow: false },
]);

let worker: Worker | null = null;
let terminals: Record<number, any> = {};

onMounted(() => {
  // Worker initialization
  worker = new Worker(new URL('./workers/ws.worker.ts', import.meta.url), {
    type: 'module',
  });
  worker.onmessage = (e) => {
    if (e.data.type === 'frame') {
      const frame = e.data.payload;
      if (frame.type === 0x06) { // SlowWarning
        for (const block of frame.sessions) {
          const session = sessions.value.find(s => s.id === block.sessionId);
          if (session) {
            session.isSlow = block.data.isSlow;
          }
        }
      }
    } else if (e.data.type === 'confirm') {
      // Handle risk confirmation request
      const { sessionId, command, message } = e.data.payload;
      riskStore.requestConfirmation(sessionId, command, message);
    } else if (e.data.type === 'closed') {
      console.log('Connection closed');
    }
  };
  worker.postMessage({ type: 'connect', payload: { url: 'ws://localhost:8080/ws' } });
});

function handleClick(id: number) {
  clientStore.selectOnly(id);
}

function handleShiftClick(id: number) {
  clientStore.toggleSelect(id);
}
</script>

<style scoped>
.toolbar {
  padding: 10px;
  background: #2d2d2d;
  color: #d4d4d4;
  margin-bottom: 10px;
}
.terminal-wrapper {
  border: 2px solid transparent;
}
.terminal-wrapper.active {
  border-color: #0066cc;
}
.terminal-wrapper.selected {
  border-color: #00cc66;
}
.terminal-wrapper.slow {
  background: rgba(255, 200, 0, 0.2);
}
</style>

<style>
.container {
  padding: 20px;
}
.terminal-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(600px, 1fr));
  gap: 10px;
  margin-top: 20px;
}
</style>