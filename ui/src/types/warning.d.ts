export interface WarningScope {
  scope: string;
  entity_type: string;
  entity: string;
}

export interface Warning {
  uuid: string;
  status: string;
  scope: WarningScope;
  type: string;
  first_seen_date: string;
  last_seen_date: string;
  updated_date: string;
  messages: string[];
  count: number;
}
