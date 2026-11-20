import React from 'react';
import { createRoot } from 'react-dom/client';
import { BrowserRouter } from 'react-router';
import App from './App.jsx';
import { appBasePath } from './lib/paths.js';
import './styles.css';

createRoot(document.getElementById('root')).render(
  <BrowserRouter basename={appBasePath() || '/'}>
    <App />
  </BrowserRouter>
);
