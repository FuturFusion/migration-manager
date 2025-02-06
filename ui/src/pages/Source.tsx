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
          content: item.name,
          sortKey: item.name
        },
        {
          content: "VMware"
        },
        {
          content: props.endpoint,
          sortKey: props.endpoint
        },
        {
          content: props.username,
          sortKey: props.username
        },
        {
          content: item.insecure.toString(),
          sortKey: item.insecure.toString()
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

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <DataTable headers={headers} rows={rows} />
      </div>
    </div>
  );
};

export default Source;
