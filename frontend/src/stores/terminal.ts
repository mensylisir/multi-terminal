import { defineStore } from 'pinia';

interface SessionState {
  isSlow: boolean;
  tuiState: boolean;
  echoLock: boolean;
}

interface TerminalState {
  sessions: Map<number, SessionState>;
}

export const useTerminalStore = defineStore('terminal', {
  state: (): TerminalState => ({
    sessions: new Map(),
  }),
  getters: {
    // Precise selector for a specific session - only re-computes when that session changes
    getSessionState: (state) => (sessionId: number): SessionState => {
      return state.sessions.get(sessionId) || { isSlow: false, tuiState: false, echoLock: false };
    },
    // Compound selectors for specific properties
    isSlow: (state) => (sessionId: number): boolean => {
      return state.sessions.get(sessionId)?.isSlow ?? false;
    },
    tuiState: (state) => (sessionId: number): boolean => {
      return state.sessions.get(sessionId)?.tuiState ?? false;
    },
    echoLock: (state) => (sessionId: number): boolean => {
      return state.sessions.get(sessionId)?.echoLock ?? false;
    },
  },
  actions: {
    setSlow(sessionId: number, isSlow: boolean) {
      const session = this.sessions.get(sessionId) || { isSlow: false, tuiState: false, echoLock: false };
      session.isSlow = isSlow;
      this.sessions.set(sessionId, session);
    },
    setTuiState(sessionId: number, tuiState: boolean) {
      const session = this.sessions.get(sessionId) || { isSlow: false, tuiState: false, echoLock: false };
      session.tuiState = tuiState;
      this.sessions.set(sessionId, session);
    },
    setEchoLock(sessionId: number, locked: boolean) {
      const session = this.sessions.get(sessionId) || { isSlow: false, tuiState: false, echoLock: false };
      session.echoLock = locked;
      this.sessions.set(sessionId, session);
    },
    getEchoLock(sessionId: number): boolean {
      return this.sessions.get(sessionId)?.echoLock ?? false;
    },
  },
});