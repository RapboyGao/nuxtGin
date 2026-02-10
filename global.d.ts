declare global {
  export function useRuntimeConfig(): {
    public: {
      ginPort?: string | number | null;
      isDevelopment?: boolean | null;
      [key: string]: unknown;
    };
  };
}

export {};
