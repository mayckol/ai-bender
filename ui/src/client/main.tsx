import { render } from 'preact';
import { LocationProvider, Route, Router } from 'preact-iso';

import { SessionList } from './pages/SessionList.tsx';
import { SessionTimeline } from './pages/SessionTimeline.tsx';

function NotFound() {
  return (
    <div class="shell">
      <div class="card">
        <h2>Not found</h2>
        <p>
          This page does not exist. <a href="/">Return to the session list</a>.
        </p>
      </div>
    </div>
  );
}

function App() {
  return (
    <LocationProvider>
      <Router>
        <Route path="/" component={SessionList} />
        <Route path="/sessions/:id" component={SessionTimeline} />
        <Route default component={NotFound} />
      </Router>
    </LocationProvider>
  );
}

const host = document.getElementById('app');
if (host) render(<App />, host);
