// logIds maps the path for a get-entries request to the internal ID of the log
// that the request is for. This is the same ID that was chosen in config.yaml.
const logIds = {
  "/ct/v1/get-entries": 1,
}
// friendlyURL is the URL we should use to download from Backblaze B2, without
// the trailing slash.
const friendlyUrl = "<omitted>"

// ~~~ Nothing below requires modifications from the operator. ~~~

function getQueryVar(search, key) {
  let pairs = search.substring(1).split("&")
  for (let i = 0; i < pairs.length; i++) {
    let pair = pairs[i].split("=")
    if (decodeURIComponent(pair[0]) == key) {
      let out = parseInt(decodeURIComponent(pair[1]), 10)
      if (isNaN(out) || out < 0) {
        throw new Error("failed to parse query variable")
      }
      return out
    }
  }
  throw new Error("query variable not found")
}

async function handleRequest(request) {
  // Parse the request. Identify which log this request is for. Extract the
  // `start` and `end` parameters and validate them.
  let u = new URL(request.url)
  let id = logIds[u.pathname]
  if (id == null) {
    throw new Error("get-entries request for unknown log")
  }
  let start = getQueryVar(u.search, "start"), end = getQueryVar(u.search, "end")
  if (start > end) {
    throw new Error("start index is greater than end index")
  }

  // Get the STH of the log, so we know the upper bound.
  let sthRes = await fetch("https://" + u.hostname + u.pathname.replace("get-entries", "get-sth"))
  if (!sthRes.ok) {
    return new Response("failed to fetch most recent sth",
      {status: 500, statusText: "Internal Server Error"})
  }
  let sth = await sthRes.json()
  if (start >= sth.tree_size) {
    return new Response("there is no leaf with that index yet",
      {status: 500, statusText: "Internal Server Error"})
  } else if (end >= sth.tree_size) {
    end = sth.tree_size - 1
  }

  // Get the batch of raw leaf data from B2.
  let leavesRes = await fetch(friendlyUrl + "/leaves-" + id.toString() + "/" + Math.floor(start/1024).toString(16))
  if (!leavesRes.ok) {
    return new Response("failed to fetch leaves from backend",
      {status: 500, statusText: "Internal Server Error"})
  }
  let leaves = await leavesRes.json()

  let out = []
  for (let i = start%1024; i < leaves.length && out.length <= end-start; i++) {
    out.push({leaf_input: leaves[i].leaf_value, extra_data: leaves[i].extra_data})
  }
  return new Response(JSON.stringify({entries: out}))
}

addEventListener("fetch", event => {
  event.respondWith(handleRequest(event.request))
})
