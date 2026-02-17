local NuiSplit = require("nui.split")
local utils = require("architect.utils")

local M = {}

function M.create()
  local panel = NuiSplit({
    relative = "editor",
    position = "bottom",
    size = 12,
    buf_options = {
      modifiable = false,
      filetype = "architect-chat",
    },
  })
  M.panel = panel
  M.messages = {}
  return panel
end

function M.append(message)
  table.insert(M.messages, message)
  M.render()
end

function M.render()
  if not M.panel then return end
  local buf = M.panel.bufnr
  if not buf or not vim.api.nvim_buf_is_valid(buf) then return end
  vim.bo[buf].modifiable = true

  local lines = { "  AGENT CHAT ROOM", "  ───────────────" }
  local start = math.max(1, #M.messages - 50)
  for i = start, #M.messages do
    local msg = M.messages[i]
    local prefix = utils.type_prefix[msg.msg_type] or "[???]"
    local agent_short = utils.short_id(msg.agent_id)
    table.insert(lines, string.format("  %s %s: %s", prefix, agent_short, msg.content))
  end

  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modifiable = false

  -- Auto-scroll to bottom
  local win = M.panel.winid
  if win and vim.api.nvim_win_is_valid(win) then
    vim.api.nvim_win_set_cursor(win, { #lines, 0 })
  end
end

return M
