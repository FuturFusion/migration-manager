import { Routes, Route } from "react-router";
import { Container } from 'react-bootstrap';
import Sidebar from "./components/Sidebar";
import Batch from "./pages/Batch.tsx"
import Instance from "./pages/Instance.tsx"
import Source from "./pages/Source.tsx"
import Target from "./pages/Target.tsx"
import Queue from "./pages/Queue.tsx"

function App() {
  return (
    <>
    <div style={{ display: 'flex' }}>
      <Sidebar />
      <Container fluid style={{ paddingLeft: '30px', paddingTop: '30px', transition: 'padding-left 0.3s' }}>
        <Routes>
          <Route index path="/ui/sources" element={<Source />} />
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
