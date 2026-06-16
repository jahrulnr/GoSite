import { render } from 'preact';
import { App } from './App';
import { AppProvider } from './lib/store';
import { TerminalProvider } from './lib/terminalStore';
import './styles/theme.css';
import './styles/components.css';
import './styles/codemirror-theme.css';
import './styles/terminal.css';

render(
  <AppProvider>
    <TerminalProvider>
      <App />
    </TerminalProvider>
  </AppProvider>,
  document.getElementById('app')!,
);
