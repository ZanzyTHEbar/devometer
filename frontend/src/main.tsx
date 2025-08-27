import { render } from "solid-js/web";
import { onMount } from "solid-js";
import "./styles.css";
import App from "./App";

// Enable SolidJS dev tools in development
if (import.meta.env.DEV) {
  // This will enable better debugging in development
  console.log("Running in development mode");
}

const root = document.getElementById("root");

if (!root) {
  throw new Error("Root element not found");
}

// Render the app
render(() => <App />, root);

// Cleanup on page unload (optional but good practice)
onMount(() => {
  const cleanup = () => {
    // Any cleanup logic here
  };

  window.addEventListener("beforeunload", cleanup);

  return () => {
    window.removeEventListener("beforeunload", cleanup);
  };
});
