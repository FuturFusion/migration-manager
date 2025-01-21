import { useNavigate } from 'react-router';
import { useParams } from 'react-router';
import BatchInstances from 'components/BatchInstances';
import BatchOverview from 'components/BatchOverview';
import TabView from 'components/TabView';

const BatchDetail = () => {
  const { name, activeTab }  = useParams();
  const navigate = useNavigate();

  const tabs = [
    {
      key: 'overview',
      title: 'Overview',
      content: <BatchOverview />
    },
    {
      key: 'instances',
      title: 'Instances',
      content: <BatchInstances />
    },
  ];

  return (
    <TabView
      defaultTab='overview'
      activeTab={ activeTab }
      tabs={ tabs }
      onSelect={(key) => navigate(`/ui/batches/${name}/${key}`)} />
  );
};

export default BatchDetail;
