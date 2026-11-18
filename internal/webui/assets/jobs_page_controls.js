// The Recent jobs page is server-rendered and has no application script, so
// this gives it the helpers web_app_stickies.js expects (the same as
// web_app_core.js has, for the mount of a basedir, fetch, and dates) and the
// Launches button, which opens the application page's Launches view.
const mountedBasedirID = (() => {
  const parts = location.pathname.split("/").filter(Boolean);
  return parts[0] === "_basedir" ? parts[1] : "";
})();
function appURL(path) {
  if (!path.startsWith("/")) path = "/" + path;
  return mountedBasedirID && !path.startsWith("/_basedir/")
    ? "/_basedir/" + mountedBasedirID + path
    : path;
}
const nativeFetch = window.fetch.bind(window);
window.fetch = function (input, options) {
  if (typeof input === "string" && input.startsWith("/")) input = appURL(input);
  return nativeFetch(input, options);
};
function pageParts() {
  const parts = location.pathname.split("/").filter(Boolean);
  if (parts[0] === "_basedir") parts.splice(0, 2);
  return parts;
}
async function fetchWebJSON(path) {
  try {
    const response = await fetch(path);
    return response.ok ? await response.json() : null;
  } catch (error) {
    return null;
  }
}
/** A local date and time as the application pages write them. */
function formatDateTime(value) {
  const date = new Date(value);
  if (Number.isNaN(+date)) return "";
  const two = (number) => String(number).padStart(2, "0");
  return (
    date.getFullYear() +
    "-" +
    two(date.getMonth() + 1) +
    "-" +
    two(date.getDate()) +
    " " +
    two(date.getHours()) +
    ":" +
    two(date.getMinutes()) +
    ":" +
    two(date.getSeconds())
  );
}
async function showJobsPageLaunches() {
  const button = document.getElementById("launches-button");
  if (!button) return;
  button.addEventListener("click", () => {
    location.href = appURL("/") + "?launches=1";
  });
  const count = async () => {
    try {
      const response = await fetch("/api/launches", { cache: "no-store" });
      const body = await response.json();
      if (!response.ok || !Array.isArray(body.launches)) throw new Error();
      const live = body.launches.filter((launch) =>
        ["starting", "running", "stopping"].includes(launch.state),
      );
      button.textContent = "Launches (" + live.length + ")";
    } catch (error) {
      button.textContent = "Launches (?)";
    }
  };
  try {
    const response = await fetch("/api/launchers");
    const availability = response.ok ? await response.json() : null;
    button.hidden = !(availability && availability.available);
  } catch (error) {
    button.hidden = true;
  }
  if (!button.hidden) {
    count();
    setInterval(count, 2000);
  }
}
showJobsPageLaunches();
