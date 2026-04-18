import { defineStore } from 'pinia';

interface PendingConfirmation {
  sessionId: number;
  command: string;
  message: string;
  resolve: (confirmed: boolean) => void;
}

interface RiskState {
  pendingConfirmations: Map<number, PendingConfirmation>;
}

export const useRiskStore = defineStore('risk', {
  state: (): RiskState => ({
    pendingConfirmations: new Map(),
  }),
  actions: {
    requestConfirmation(sessionId: number, command: string, message: string): Promise<boolean> {
      return new Promise((resolve) => {
        this.pendingConfirmations.set(sessionId, {
          sessionId,
          command,
          message,
          resolve,
        });
      });
    },
    resolveConfirmation(sessionId: number, confirmed: boolean) {
      const pending = this.pendingConfirmations.get(sessionId);
      if (pending) {
        pending.resolve(confirmed);
        this.pendingConfirmations.delete(sessionId);
      }
    },
    cancelConfirmation(sessionId: number) {
      const pending = this.pendingConfirmations.get(sessionId);
      if (pending) {
        pending.resolve(false);
        this.pendingConfirmations.delete(sessionId);
      }
    },
    hasPendingConfirmation(sessionId: number): boolean {
      return this.pendingConfirmations.has(sessionId);
    },
  },
});
