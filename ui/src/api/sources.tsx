import { Source } from 'types/source';

export const fetchSources = (): Promise<Source[]> => {
  return new Promise((resolve, reject) => {
    fetch(`/1.0/sources?recursion=1`)
      .then((response) => response.json())
      .then((data) => resolve(data.metadata))
      .catch(reject);
  });
};
