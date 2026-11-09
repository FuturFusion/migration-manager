export interface WorkerLogSession {
  session: string;
  size: number;
  modified: string;
}

export interface WorkerLogContent {
  content: string;
  size: number;
  truncated: boolean;
}
