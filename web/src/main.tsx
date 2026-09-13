import React from "react";
import ReactDOM from "react-dom/client";
import { BrowserRouter, Routes, Route } from "react-router-dom";
import { QueryClient, QueryClientProvider } from "@tanstack/react-query";
import Layout from "./components/Layout";
import WorkbenchPage from "./pages/WorkbenchPage";
import TTSPage from "./pages/TTSPage";
import TTSLongPage from "./pages/TTSLongPage";
import TTSStreamPage from "./pages/TTSStreamPage";
import ASRPage from "./pages/ASRPage";
import PodcastPage from "./pages/PodcastPage";
import SeparatePage from "./pages/SeparatePage";
import HistoryPage from "./pages/HistoryPage";
import SettingsPage from "./pages/SettingsPage";
import { ToastProvider } from "./ui";
// 自托管字体（离线可用，不依赖 Google CDN）；仅 latin 子集，中文走系统回退
import "@fontsource/fira-sans/latin-400.css";
import "@fontsource/fira-sans/latin-500.css";
import "@fontsource/fira-sans/latin-600.css";
import "@fontsource/fira-code/latin-400.css";
import "@fontsource/fira-code/latin-500.css";
import "./theme.css";

const qc = new QueryClient({
  defaultOptions: { queries: { refetchOnWindowFocus: false, staleTime: 10_000 } },
});

ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <QueryClientProvider client={qc}>
      <ToastProvider>
        <BrowserRouter>
          <Routes>
            <Route element={<Layout />}>
              <Route path="/" element={<WorkbenchPage />} />
              <Route path="/tts" element={<TTSPage />} />
              <Route path="/tts-long" element={<TTSLongPage />} />
              <Route path="/tts-stream" element={<TTSStreamPage />} />
              <Route path="/asr" element={<ASRPage />} />
              <Route path="/podcast" element={<PodcastPage />} />
              <Route path="/separate" element={<SeparatePage />} />
              <Route path="/history" element={<HistoryPage />} />
              <Route path="/settings" element={<SettingsPage />} />
            </Route>
          </Routes>
        </BrowserRouter>
      </ToastProvider>
    </QueryClientProvider>
  </React.StrictMode>
);
