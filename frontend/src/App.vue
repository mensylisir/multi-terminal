<template>
  <div class="container">
    <h1>Multi-Terminal</h1>
    <div class="status">{{ connected ? '已连接' : '未连接' }}</div>
    <button @click="connect">连接</button>
  </div>
</template>

<script setup lang="ts">
import { ref } from 'vue';

const connected = ref(false);
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
}
</script>

<style>
.container {
  padding: 20px;
}
.status {
  margin: 10px 0;
}
</style>