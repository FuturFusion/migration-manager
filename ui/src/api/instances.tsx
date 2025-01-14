import { Instance } from 'types/instance';

export const fetchInstances = (): Promise<Instance[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/instances?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};
