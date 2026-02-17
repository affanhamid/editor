local NuiSplit = require("nui.split")
local utils = require("architect.utils")

local M = {}

function M.create()
  local panel = NuiSplit({
    relative = "editor",
    position = "right",
    size = 40,
    buf_options = {
      modifiable = false,
      filetype = "architect-swarm",
    },
  })
  M.panel = panel
  return panel
end

function M.update(agents)
  if not M.panel then return end
  local buf = M.panel.bufnr
  if not buf or not vim.api.nvim_buf_is_valid(buf) then return end
  vim.bo[buf].modifiable = true

  local lines = { "  AGENT SWARM", "  ───────────" }
  for _, agent in ipairs(agents) do
    local icon = utils.status_icons[agent.status] or "?"
    local task_info = agent.current_task_id and (" → task #" .. agent.current_task_id) or ""
    table.insert(lines, string.format("  %s %s%s", icon, utils.short_id(agent.agent_id), task_info))
  end

  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  vim.bo[buf].modifiable = false

  -- Apply highlights
  for i, agent in ipairs(agents) do
    local hl = utils.status_hl[agent.status] or "Normal"
    vim.api.nvim_buf_add_highlight(buf, -1, hl, i + 1, 2, 3)
  end
end

return M
