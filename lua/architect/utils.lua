local M = {}

M.status_icons = {
  starting = "◐",
  working  = "●",
  idle     = "○",
  blocked  = "■",
  dead     = "✗",
}

M.status_hl = {
  starting = "DiagnosticInfo",
  working  = "DiagnosticOk",
  idle     = "Comment",
  blocked  = "DiagnosticWarn",
  dead     = "DiagnosticError",
}

M.task_icons = {
  pending     = "○",
  in_progress = "●",
  completed   = "✓",
  failed      = "✗",
  blocked     = "■",
}

M.type_prefix = {
  update    = "[UPD]",
  question  = "[ASK]",
  answer    = "[ANS]",
  blocker   = "[BLK]",
  discovery = "[DSC]",
  decision  = "[DEC]",
}

function M.truncate(str, max_len)
  if #str > max_len then
    return str:sub(1, max_len - 3) .. "..."
  end
  return str
end

function M.short_id(id)
  if id and #id > 8 then
    return id:sub(1, 8)
  end
  return id or "unknown"
end

return M
