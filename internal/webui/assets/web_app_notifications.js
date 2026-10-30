function notificationsSupported() {
  return typeof Notification !== "undefined";
}
const notificationIconURL = "__ROTARI_NOTIFICATION_ICON__";
const notificationsDefaultOn = __ROTARI_NOTIFICATION_DEFAULT__;
const notificationsEnabledKey = "rotari-notifications-enabled";
function notificationsEnabled() {
  const stored = localStorage.getItem(notificationsEnabledKey);
  if (stored === null) return notificationsDefaultOn;
  return stored !== "false";
}
function setNotificationsEnabled(enabled) {
  localStorage.setItem(notificationsEnabledKey, enabled ? "true" : "false");
}
function updateNotifyToggleLabel() {
  const button = document.getElementById("notify-toggle");
  if (!button) return;
  const reason = notificationUnavailableReason();
  if (!reason && !notificationsSupported()) {
    button.hidden = true;
    return;
  }
  button.disabled = reason !== "";
  button.title = reason;
  const enabled =
    !reason && Notification.permission === "granted" && notificationsEnabled();
  button.textContent = enabled ? "Notification on" : "Notification off";
  button.classList.toggle("notifications-on", enabled);
  button.setAttribute("aria-pressed", String(enabled));
}
async function toggleRunNotifications() {
  if (!notificationsSupported()) return;
  if (Notification.permission === "default") {
    await Notification.requestPermission();
  } else if (Notification.permission === "granted") {
    setNotificationsEnabled(!notificationsEnabled());
  }
  updateNotifyToggleLabel();
}
// The basedirs the inbox reads: the page's own and the others whose bell is
// on.
function notificationBasedirIDs() {
  const registered = new Set(registeredBasedirs.map((entry) => entry.id));
  const ids = [...(selectedNotificationBasedirIDs() || [])].filter((id) =>
    registered.has(id),
  );
  const own =
    mountedBasedirID || registeredBasedirs.find((entry) => entry.current)?.id;
  if (own) ids.push(own);
  return ids;
}
function showDesktopNotification(item) {
  if (
    !notificationsSupported() ||
    notificationUnavailableReason() ||
    Notification.permission !== "granted" ||
    !notificationsEnabled()
  )
    return;
  const notification = new Notification(item.title, {
    body: item.body,
    icon: notificationIconURL,
  });
  if (item.url)
    notification.onclick = () => {
      window.focus();
      location.href = item.url;
    };
}
updateNotifyToggleLabel();
if (typeof rewriteStaticLinks !== "function")
  window.initializeNotificationInbox({
    basedirIDs: notificationBasedirIDs,
    desktop: showDesktopNotification,
  });
