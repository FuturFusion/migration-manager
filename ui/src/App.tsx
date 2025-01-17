import { Routes, Route } from 'react-router';
import { Container } from 'react-bootstrap';
import Sidebar from 'components/Sidebar';
import Batch from 'pages/Batch';
import Home from 'pages/Home';
import Instance from 'pages/Instance';
import Source from 'pages/Source';
import Target from 'pages/Target';
import Queue from 'pages/Queue';
import { useAuth } from 'context/auth';

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
    <div style={{ display: 'flex' }}>
      <Sidebar />
      <Container fluid style={{ paddingLeft: '30px', paddingTop: '30px', transition: 'padding-left 0.3s' }}>
        <Routes>
          <Route path="/ui" element={<Home />} />
          <Route path="/ui/sources" element={<Source />} />
          <Route path="/ui/targets" element={<Target />} />
          <Route path="/ui/instances" element={<Instance />} />
          <Route path="/ui/batches" element={<Batch />} />
          <Route path="/ui/queue" element={<Queue />} />
        </Routes>
      </Container>
    </div>
    </>
  )
}

export default App
