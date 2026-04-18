import { defineStore } from 'pinia';

interface ClientState {
  activeSessionId: number | null;
  selectedSessionIds: number[];
  mode: 'single' | 'multi';
}

export const useClientStore = defineStore('client', {
  state: (): ClientState => ({
    activeSessionId: null,
    selectedSessionIds: [],
    mode: 'single',
  }),
  actions: {
    setActive(sessionId: number) {
      this.activeSessionId = sessionId;
    },
    toggleSelect(sessionId: number) {
      const idx = this.selectedSessionIds.indexOf(sessionId);
      if (idx >= 0) {
        this.selectedSessionIds.splice(idx, 1);
      } else {
        this.selectedSessionIds.push(sessionId);
      }
      this.mode = this.selectedSessionIds.length > 1 ? 'multi' : 'single';
    },
    selectOnly(sessionId: number) {
      this.selectedSessionIds = [sessionId];
      this.activeSessionId = sessionId;
      this.mode = 'single';
    },
    clearSelection() {
      this.selectedSessionIds = [];
      this.mode = 'single';
    },
    isSelected(sessionId: number): boolean {
      return this.selectedSessionIds.includes(sessionId);
    },
  },
});