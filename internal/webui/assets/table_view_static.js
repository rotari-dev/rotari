// A static snapshot of the Table viewer (rotari tableview --static-dir)
// answers the page's API requests from __ROTARI_TABLE_VIEW_STATIC__, which
// table_view_static.go embeds: every row, each column's order, Describe
// over the whole file, and the copied media files. What only the live
// server can do, such as filtering, is refused with the reason.

(function () {
  const snapshot = window.__ROTARI_TABLE_VIEW_STATIC__;
  const liveFetch = window.fetch.bind(window);

  function json(value) {
    return new Response(JSON.stringify(value), {
      headers: { "Content-Type": "application/json" },
    });
  }

  function refuse(status, message) {
    return new Response(message, { status });
  }

  // unsupported names the first parameter that asks for something other
  // than the snapshot shows, or "".
  function unsupported(query) {
    if (query.get("filters") && query.get("filters") !== "[]")
      return "Filters are not available in a static snapshot.";
    if (query.has("sep") && query.get("sep") !== snapshot.info.separator)
      return "The separator cannot be changed in a static snapshot.";
    if (query.has("header") && query.get("header") !== "1")
      return "The header row cannot be changed in a static snapshot.";
    const base = query.get("base") || "";
    if (base && base !== snapshot.info.bases[0].id)
      return "Media paths resolve only from " + snapshot.info.bases[0].label;
    return "";
  }

  // withMediaURLs is a row whose media cells link their copies.
  function withMediaURLs(row) {
    if (!row.media) return row;
    const media = {};
    Object.entries(row.media).forEach(([cell, value]) => {
      const copy = snapshot.media[value.ref];
      media[cell] = copy ? Object.assign({}, value, { url: copy }) : value;
    });
    return Object.assign({}, row, { media });
  }

  function rows(query) {
    const total = snapshot.rows.length;
    const offset = Number(query.get("offset") || "0");
    const limit = Number(query.get("limit") || "100");
    if (!Number.isInteger(offset) || offset < 0)
      return refuse(400, "offset must not be negative");
    if (!Number.isInteger(limit) || limit < 1 || limit > 1000)
      return refuse(400, "limit must be between 1 and 1000");
    let order = null;
    if (query.get("sort")) {
      const column = Number(query.get("sort"));
      const orders =
        query.get("desc") === "1" ? snapshot.descending : snapshot.ascending;
      if (!Number.isInteger(column) || !orders[column])
        return refuse(400, "sort names no column of the table");
      order = orders[column];
    }
    const numbers = [];
    for (let index = offset; index < Math.min(total, offset + limit); index++)
      numbers.push(order ? order[index] : index);
    const cut = new Set(snapshot.cut || []);
    return json({
      columns: snapshot.columns,
      rows: numbers.map((number) => withMediaURLs(snapshot.rows[number])),
      offset,
      matched: total,
      total,
      cut: numbers.some((number) => cut.has(number)) || undefined,
      notes: snapshot.notes,
    });
  }

  window.fetch = async function (input, init) {
    // The page fetches by URL string; a Request names its own.
    const request = new URL(String(input.url || input), window.location.href);
    const route = request.pathname.match(/\/api\/([a-z]+)$/);
    if (!route) return liveFetch(input, init);
    const query = request.searchParams;
    const reason = unsupported(query);
    if (reason) return refuse(400, reason);
    switch (route[1]) {
      case "info":
        return json(snapshot.info);
      case "rows":
        return rows(query);
      case "describe":
        return json(snapshot.describe);
    }
    return refuse(404, "A static snapshot has no " + request.pathname);
  };
})();
