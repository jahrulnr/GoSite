import { render } from 'preact';
import { App } from './App';
import { AppProvider } from './lib/store';
import './styles/theme.css';
import './styles/components.css';

render(
  <AppProvider>
    <App />
  </AppProvider>,
  document.getElementById('app')!,
);
