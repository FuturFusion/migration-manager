import { Routes, Route } from "react-router";
import { Container } from "react-bootstrap";
import Notification from "components/Notification";
import Sidebar from "components/Sidebar";
import { useAuth } from "context/auth";
import Batch from "pages/Batch";
import BatchCreate from "pages/BatchCreate";
import BatchDetail from "pages/BatchDetail";
import Home from "pages/Home";
import Instance from "pages/Instance";
import InstanceDetail from "pages/InstanceDetail";
import Source from "pages/Source";
import SourceCreate from "pages/SourceCreate";
import SourceDetail from "pages/SourceDetail";
import Target from "pages/Target";
import TargetCreate from "pages/TargetCreate";
import TargetDetail from "pages/TargetDetail";
import Queue from "pages/Queue";

function App() {
  const { isAuthenticated, isAuthLoading } = useAuth();

  if (isAuthLoading) {
    return <div>Loading...</div>;
  }

  if (!isAuthenticated) {
    if (window.location.pathname !== "/ui/") {
      window.location.href = "/ui/";
    }
  }

  return (
    <>
      <div style={{ display: "flex" }}>
        <Sidebar />
        <Container
          fluid
          style={{
            paddingLeft: "30px",
            paddingTop: "30px",
            transition: "padding-left 0.3s",
          }}
        >
          <Routes>
            <Route path="/ui" element={<Home />} />
            <Route path="/ui/sources" element={<Source />} />
            <Route path="/ui/sources/create" element={<SourceCreate />} />
            <Route path="/ui/sources/:name" element={<SourceDetail />} />
            <Route
              path="/ui/sources/:name/:activeTab"
              element={<SourceDetail />}
            />
            <Route path="/ui/targets" element={<Target />} />
            <Route path="/ui/targets/create" element={<TargetCreate />} />
            <Route path="/ui/targets/:name" element={<TargetDetail />} />
            <Route
              path="/ui/targets/:name/:activeTab"
              element={<TargetDetail />}
            />
            <Route path="/ui/instances" element={<Instance />} />
            <Route path="/ui/instances/:uuid" element={<InstanceDetail />} />
            <Route
              path="/ui/instances/:uuid/:activeTab"
              element={<InstanceDetail />}
            />
            <Route path="/ui/batches" element={<Batch />} />
            <Route path="/ui/batches/create" element={<BatchCreate />} />
            <Route path="/ui/batches/:name" element={<BatchDetail />} />
            <Route
              path="/ui/batches/:name/:activeTab"
              element={<BatchDetail />}
            />
            <Route path="/ui/queue" element={<Queue />} />
          </Routes>
          <Notification />
        </Container>
      </div>
    </>
  );
}

export default App;
