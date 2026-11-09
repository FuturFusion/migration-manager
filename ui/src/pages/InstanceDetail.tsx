import { useNavigate, useParams } from "react-router";
import InstanceOverview from "components/InstanceOverview";
import InstanceOverrides from "components/InstanceOverrides";
import TabView from "components/TabView";
import WorkerLogs from "components/WorkerLogs";

const InstanceDetail = () => {
  const { uuid, activeTab } = useParams<{ uuid: string; activeTab: string }>();
  const navigate = useNavigate();

  const tabs = [
    {
      key: "overview",
      title: "Overview",
      content: <InstanceOverview />,
    },
    {
      key: "overrides",
      title: "Overrides",
      content: <InstanceOverrides />,
    },
    {
      key: "logs",
      title: "Worker logs",
      content: activeTab === "logs" ? <WorkerLogs /> : null,
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) => navigate(`/ui/instances/${uuid}/${key}`)}
        />
      </div>
    </div>
  );
};

export default InstanceDetail;
