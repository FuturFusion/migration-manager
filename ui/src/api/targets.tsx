import { IncusTarget } from 'types/target';

export const fetchTargets = (): Promise<IncusTarget[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/targets?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};
