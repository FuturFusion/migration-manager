import { useQuery } from "@tanstack/react-query";
import { useNavigate, useParams } from "react-router";
import { fetchArtifact } from "api/artifacts";
import ArtifactConfiguration from "components/ArtifactConfiguration";
import ArtifactOverview from "components/ArtifactOverview";
import ArtifactFiles from "components/ArtifactFiles";
import TabView from "components/TabView";

const ArtifactDetail = () => {
  const { uuid, activeTab } = useParams<{ uuid: string; activeTab: string }>();
  const navigate = useNavigate();

  const {
    data: artifact = null,
    error,
    isLoading,
  } = useQuery({
    queryKey: ["artifacts", uuid],
    queryFn: () => fetchArtifact(uuid),
  });

  if (isLoading) {
    return <div>Loading...</div>;
  }

  if (error || !artifact) {
    return <div>Error while loading artifact</div>;
  }

  const tabs = [
    {
      key: "overview",
      title: "Overview",
      content: <ArtifactOverview />,
    },
    {
      key: "configuration",
      title: "Configuration",
      content: <ArtifactConfiguration />,
    },
    {
      key: "files",
      title: "Files",
      content: <ArtifactFiles />,
    },
  ];

  return (
    <div className="d-flex flex-column">
      <div className="scroll-container flex-grow-1 p-3">
        <TabView
          defaultTab="overview"
          activeTab={activeTab}
          tabs={tabs}
          onSelect={(key) => navigate(`/ui/artifacts/${uuid}/${key}`)}
        />
      </div>
    </div>
  );
};

export default ArtifactDetail;
