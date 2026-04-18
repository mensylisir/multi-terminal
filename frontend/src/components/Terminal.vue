<template>
  <div ref="terminalRef" class="terminal-container" :class="{ slow: isSlowSession, tui: isTuiSession }">
    <div v-if="isTuiSession" class="tui-overlay">
      <span>环境异常锁定</span>
    </div>
  </div>
</template>

<script setup lang="ts">
import { ref, computed, onMounted, onUnmounted } from 'vue';
import { Terminal } from '@xterm/xterm';
import { WebglAddon } from '@xterm/addon-webgl';
import { useTerminalStore } from '../stores/terminal';
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
let webglEnabled = false;

const terminalStore = useTerminalStore();

// Precise selectors - only re-compute when this specific session changes
const isSlowSession = computed(() => terminalStore.isSlow(props.sessionId));
const isTuiSession = computed(() => terminalStore.tuiState(props.sessionId));

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

  // Attempt WebGL with proper addon configuration
  try {
    webglAddon = new WebglAddon();
    webglAddon.onContextLoss(() => {
      console.warn('WebGL context lost, falling back to canvas');
      webglEnabled = false;
      if (webglAddon) {
        webglAddon.dispose();
        webglAddon = null;
      }
    });

    // Load addon - if this succeeds, WebGL is available
    terminal.loadAddon(webglAddon);
    webglEnabled = true;
    console.log('WebGL rendering enabled');

  } catch (e) {
    console.warn('WebGL not available, falling back to canvas:', e);
    if (webglAddon) {
      webglAddon.dispose();
      webglAddon = null;
    }
  }

  terminal.open(terminalRef.value);

  // Fallback if WebGL context doesn't initialize
  if (!webglEnabled && !webglAddon) {
    console.log('Using Canvas renderer');
  }
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
  if (webglAddon) {
    webglAddon.dispose();
  }
  terminal?.dispose();
});
</script>

<style scoped>
.terminal-container {
  width: 100%;
  height: 100%;
  min-height: 400px;
  position: relative;
}

.terminal-container.slow {
  background: rgba(255, 200, 0, 0.2);
}

.tui-overlay {
  position: absolute;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background: rgba(255, 0, 0, 0.3);
  display: flex;
  align-items: center;
  justify-content: center;
  color: white;
  font-weight: bold;
  z-index: 10;
}
</style>