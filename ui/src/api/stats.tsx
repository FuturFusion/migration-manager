import { StatsOverview } from "types/stats";

export const fetchStats = (): Promise<StatsOverview> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/stats`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};
