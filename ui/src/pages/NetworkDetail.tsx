import { useNavigate, useParams, useSearchParams } from "react-router";
import NetworkOverview from "components/NetworkOverview";
import NetworkOverrides from "components/NetworkOverrides";
import TabView from "components/TabView";

const NetworkDetail = () => {
  const { name, activeTab } = useParams<{ name: string; activeTab: string }>();
  const navigate = useNavigate();
  const [searchParams] = useSearchParams();
  const source = searchParams.get("source");

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
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) =>
            navigate(`/ui/networks/${name}/${key}?source=${source}`)
          }
        />
      </div>
    </div>
  );
};

export default NetworkDetail;
