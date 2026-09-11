import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Layout from "./components/Layout";
import WorkbenchPage from "./pages/WorkbenchPage";
import TTSPage from "./pages/TTSPage";
import HistoryPage from "./pages/HistoryPage";
import "./theme.css";

const qc = new QueryClient();

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <Routes>
          <Route element={<Layout />}>
            <Route path="/" element={<WorkbenchPage />} />
            <Route path="/tts" element={<TTSPage />} />
            <Route path="/history" element={<HistoryPage />} />
          </Route>
        </Routes>
      </BrowserRouter>
    </QueryClientProvider>
  </React.StrictMode>
);
