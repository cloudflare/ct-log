// logIds maps the path for a get-entries request to the internal ID of the log
// that the request is for. This is the same ID that was chosen in config.yaml.
const logIds = {
  "/ct/v1/get-entries": 1,
}
// friendlyURL is the URL we should use to download from Backblaze B2, without
// the trailing slash. It should likely be the same as `b2_url` in your config.
const friendlyUrl = "<omitted>"

// ~~~ Nothing below requires modifications from the operator. ~~~

function getBounds(search) {
  let start = -1, end = -1

  let pairs = search.substring(1).split("&")
  for (let i = 0; i < pairs.length; i++) {
    let pair = pairs[i].split("=")

    let key = decodeURIComponent(pair[0])
    if (key != "start" && key != "end") {
      continue
    }
    let val = parseInt(decodeURIComponent(pair[1]), 10)
    if (isNaN(val) || val < 0) {
      throw new Error("failed to parse query variable")
    }

    if (key == "start") {
      start = val
    } else if (key == "end") {
      end = val
    }
  }

  if (start == -1 || end == -1) {
    throw new Error("query variable not found")
  }
  return {start: start, end: end}
}

function transformJSON(bounds, leaves) {
  let out = ["{\"entries\":["]

  // Manually, and very lazily parse -> re-format -> re-serialize the JSON.
  // Actual JSON parsing/serializing is too slow to stay within CPU budget.
  leaves = leaves.slice(1, -1).split("},{")

  let startIdx = bounds.start%1024, comma = false
  for (let i = startIdx; i < leaves.length && i-startIdx <= bounds.end-bounds.start; i++) {
    if (comma) {
      out.push(",")
    } else {
      comma = true
    }

    let leaf = leaves[i]
    if (i == 0) {
      leaf = leaf.slice(1)
    } else if (i == leaves.length-1) {
      leaf = leaf.slice(0, -1)
    }

    let entry = leaf.split(",")
      .filter((part) => {
        return part.startsWith("\"leaf_value\":") || part.startsWith("\"extra_data\":")
      })
      .join(",")
      .replace("\"leaf_value\":", "\"leaf_input\":")
    out.push("{" + entry + "}")
  }

  out.push("]}")
  return out.join("")
}

async function handleRequest(request) {
  // Parse the request. Identify which log this request is for. Extract the
  // `start` and `end` parameters and validate them.
  let u = new URL(request.url)
  let id = logIds[u.pathname]
  if (id == null) {
    throw new Error("get-entries request for unknown log")
  }
  let bounds = getBounds(u.search)
  if (bounds.start > bounds.end) {
    throw new Error("start index is greater than end index")
  }

  // Get the STH of the log, so we know the upper bound.
  let tag = Math.floor((new Date()).getTime() / 10000).toString() // Don't hold on to stale STHs for too long.
  let sthRes = await fetch("https://" + u.hostname + u.pathname.replace("get-entries", "get-sth") + "?tag=" + tag)
  if (!sthRes.ok) {
    return new Response("failed to fetch most recent sth",
      {status: 500, statusText: "Internal Server Error"})
  }
  let sth = await sthRes.json()
  if (bounds.start >= sth.tree_size) {
    return new Response("there is no leaf with that index yet",
      {status: 500, statusText: "Internal Server Error"})
  } else if (bounds.end >= sth.tree_size) {
    bounds.end = sth.tree_size - 1
  }

  // Get the batch of raw leaf data from B2.
  let leavesRes = await fetch(friendlyUrl + "/leaves-" + id.toString() + "/" + Math.floor(bounds.start/1024).toString(16))
  if (!leavesRes.ok) {
    return new Response("failed to fetch leaves from backend",
      {status: 500, statusText: "Internal Server Error"})
  }
  let leaves = await leavesRes.text()

  return new Response(transformJSON(bounds, leaves))
}

addEventListener("fetch", event => {
  event.passThroughOnException()
  event.respondWith(handleRequest(event.request))
})
