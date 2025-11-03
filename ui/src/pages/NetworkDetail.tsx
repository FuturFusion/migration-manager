import { useNavigate, useParams } from "react-router";
import NetworkInstances from "components/NetworkInstances";
import NetworkOverview from "components/NetworkOverview";
import NetworkOverrides from "components/NetworkOverrides";
import TabView from "components/TabView";

const NetworkDetail = () => {
  const { uuid, activeTab } = useParams<{ uuid: string; activeTab: string }>();
  const navigate = useNavigate();

  const tabs = [
    {
      key: "overview",
      title: "Overview",
      content: <NetworkOverview />,
    },
    {
      key: "overrides",
      title: "Overrides",
      content: <NetworkOverrides />,
    },
    {
      key: "instances",
      title: "Instances",
      content: <NetworkInstances />,
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) => navigate(`/ui/networks/${uuid}/${key}`)}
        />
      </div>
    </div>
  );
};

export default NetworkDetail;
