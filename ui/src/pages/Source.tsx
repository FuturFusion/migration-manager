import { useQuery } from '@tanstack/react-query';
import { fetchSources } from 'api/sources';
import DataTable from 'components/DataTable';
import { VMwareProperties } from 'types/source';

enum SourceType {
  Unknown,
  Common,
  VMware,
}

const Source = () => {

  const {
    data: sources = [],
    error,
    isLoading,
  } = useQuery({ queryKey: ['sources'], queryFn: fetchSources })

  const headers = ["Name", "Type", "Endpoint", "Username", "Insecure"];
  const rows = sources.map((item) => {
    if (item.source_type == SourceType.VMware) {
      const props = item.properties as VMwareProperties;
      return [
        {
          content: item.name
        },
        {
          content: "VMware"
        },
        {
          content: props.endpoint
        },
        {
          content: props.username
        },
        {
          content: item.insecure.toString()
        }];
    }

    return [{content:""},{content:""},{content:""},{content:""},{content:""}];
  });

  if (isLoading) {
    return (
      <div>Loading sources...</div>
    );
  }

  if (error) {
    return (
      <div>Error while loading sources</div>
    );
  }

  return <DataTable headers={headers} rows={rows} />;
};

export default Source;
