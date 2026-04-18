<template>
  <div ref="terminalRef" class="terminal-container"></div>
</template>

<script setup lang="ts">
import { ref, onMounted, onUnmounted } from 'vue';
import { Terminal } from '@xterm/xterm';
import { WebglAddon } from '@xterm/addon-webgl';
import '@xterm/xterm/css/xterm.css';

const props = defineProps<{
  sessionId: number;
}>();

const emit = defineEmits<{
  (e: 'resize', payload: { sessionId: number; cols: number; rows: number }): void;
}>();

const terminalRef = ref<HTMLElement | null>(null);
let terminal: Terminal | null = null;
let webglAddon: WebglAddon | null = null;

// rAF throttling
let outputQueue: string[] = [];
let rafId: number | null = null;

function scheduleFlush() {
  if (rafId !== null) return;
  rafId = requestAnimationFrame(() => {
    if (outputQueue.length > 0 && terminal) {
      terminal.write(outputQueue.join(''));
      outputQueue = [];
    }
    rafId = null;
  });
}

function queueOutput(data: string) {
  outputQueue.push(data);
  scheduleFlush();
}

function initTerminal() {
  if (!terminalRef.value) return;

  terminal = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    fontFamily: 'Menlo, Monaco, "Courier New", monospace',
    theme: {
      background: '#1e1e1e',
      foreground: '#d4d4d4',
    },
    rows: 24,
    cols: 80,
    scrollback: 5000,
  });

  try {
    webglAddon = new WebglAddon();
    terminal.loadAddon(webglAddon);
  } catch (e) {
    console.warn('WebGL not available, falling back to canvas');
  }

  terminal.open(terminalRef.value);
}

function handleResize() {
  if (terminalRef.value) {
    const cols = Math.floor(terminalRef.value.clientWidth / 9);
    const rows = Math.floor(terminalRef.value.clientHeight / 17);
    terminal?.resize(cols, rows);
    emit('resize', { sessionId: props.sessionId, cols, rows });
  }
}

function writeToTerminal(data: string) {
  queueOutput(data);
}

defineExpose({ writeToTerminal, resize: handleResize });

onMounted(() => {
  initTerminal();
  window.addEventListener('resize', handleResize);
});

onUnmounted(() => {
  window.removeEventListener('resize', handleResize);
  terminal?.dispose();
  webglAddon?.dispose();
});
</script>

<style scoped>
.terminal-container {
  width: 100%;
  height: 100%;
  min-height: 400px;
}
</style>