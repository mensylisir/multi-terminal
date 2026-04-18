<template>
  <div class="container">
    <h1>Multi-Terminal</h1>
    <div class="status">{{ connected ? '已连接' : '未连接' }}</div>
    <button @click="connect">连接</button>
    <div class="terminal-grid">
      <Terminal :sessionId="1" ref="term1" />
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import Terminal from './components/Terminal.vue';

const connected = ref(false);
const term1 = ref<InstanceType<typeof Terminal> | null>(null);
let worker: Worker | null = null;

function connect() {
  worker = new Worker(new URL('./workers/ws.worker.ts', import.meta.url), {
    type: 'module',
  });
  worker.onmessage = (e) => {
    if (e.data.type === 'frame') {
      console.log('Received frame:', e.data.payload);
    } else if (e.data.type === 'closed') {
      connected.value = false;
    }
  };
  worker.postMessage({ type: 'connect', payload: { url: 'ws://localhost:8080/ws' } });
  connected.value = true;
  // 模拟输出
  setTimeout(() => {
    term1.value?.writeToTerminal('Welcome to Multi-Terminal\r\n$ ');
  }, 100);
}
</script>

<style>
.container {
  padding: 20px;
}
.status {
  margin: 10px 0;
  font-weight: bold;
}
.terminal-grid {
  display: grid;
  grid-template-columns: repeat(auto-fit, minmax(600px, 1fr));
  gap: 10px;
  margin-top: 20px;
}
button {
  padding: 8px 16px;
  background: #0066cc;
  color: white;
  border: none;
  cursor: pointer;
}
</style>