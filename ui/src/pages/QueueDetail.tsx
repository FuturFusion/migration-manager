import { useNavigate, useParams } from "react-router";
import QueueOverview from "components/QueueOverview";
import InstanceOverview from "components/InstanceOverview";
import InstanceOverrides from "components/InstanceOverrides";
import TabView from "components/TabView";

const QueueDetail = () => {
  const { uuid, activeTab } = useParams<{ uuid: string; activeTab: string }>();
  const navigate = useNavigate();

  const tabs = [
    {
      key: "overview",
      title: "Overview",
      content: <QueueOverview />,
    },
    {
      key: "instance",
      title: "Instance",
      content: <InstanceOverview />,
    },
    {
      key: "overrides",
      title: "Overrides",
      content: <InstanceOverrides />,
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) => navigate(`/ui/queue/${uuid}/${key}`)}
        />
      </div>
    </div>
  );
};

export default QueueDetail;
