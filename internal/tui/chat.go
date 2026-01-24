package tui

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"
	"github.com/charmbracelet/lipgloss"
	"github.com/cloudwego/eino/schema"
	"github.com/wwwzy/CentAgent/internal/agent"
	"github.com/wwwzy/CentAgent/internal/ui"
)

type ChatUI struct{}

func (u *ChatUI) Run(ctx context.Context, backend ui.ChatBackend, initial agent.AgentState, opts ui.ChatOptions) error {
	m := newChatModel(ctx, backend, initial, opts)
	p := tea.NewProgram(m, tea.WithAltScreen())
	_, err := p.Run()
	return err
}

type backendResultMsg struct {
	state     agent.AgentState
	err       error
	prevCount int
}

type streamTickMsg struct{}
type cancelMsg struct{}

var stdioMu sync.Mutex

type chatModel struct {
	ctx     context.Context
	backend ui.ChatBackend
	opts    ui.ChatOptions

	state agent.AgentState

	width  int
	height int

	viewport   viewport.Model
	input      textinput.Model
	spinner    spinner.Model
	thinking   bool
	followTail bool

	confirmVisible bool
	confirmTitle   string
	confirmIndex   int

	overrideContent map[int]string
	streaming       bool
	streamIdx       int
	streamPos       int
	streamFull      string

	renderer *glamour.TermRenderer

	lastInvokePrevCount int
}

func newChatModel(ctx context.Context, backend ui.ChatBackend, initial agent.AgentState, opts ui.ChatOptions) chatModel {
	state := initial
	if state.Context == nil {
		state.Context = map[string]interface{}{}
	}

	s := spinner.New()
	s.Spinner = spinner.MiniDot

	ti := textinput.New()
	ti.Placeholder = "输入消息，回车发送"
	ti.Prompt = ""
	ti.Focus()

	vp := viewport.New(0, 0)
	vp.SetContent("")

	return chatModel{
		ctx:             ctx,
		backend:         backend,
		opts:            opts,
		state:           state,
		viewport:        vp,
		input:           ti,
		spinner:         s,
		followTail:      true,
		overrideContent: map[int]string{},
	}
}

func (m chatModel) Init() tea.Cmd {
	return tea.Batch(textinput.Blink, m.spinner.Tick, waitCancel(m.ctx))
}

func waitCancel(ctx context.Context) tea.Cmd {
	return func() tea.Msg {
		<-ctx.Done()
		return cancelMsg{}
	}
}

func (m chatModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case cancelMsg:
		return m, tea.Quit

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

		inputHeight := 3
		footerHeight := 1
		chatHeight := m.height - inputHeight - footerHeight
		if chatHeight < 1 {
			chatHeight = 1
		}

		m.viewport.Width = m.width
		m.viewport.Height = chatHeight

		m.input.Width = max(10, m.width-4)

		m.resetMarkdownRenderer()
		m.updateViewportContent(m.renderChat())
		return m, nil

	case spinner.TickMsg:
		if m.thinking {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case backendResultMsg:
		m.thinking = false
		if msg.err != nil {
			m.state.Messages = append(m.state.Messages, &schema.Message{
				Role:    schema.Assistant,
				Content: fmt.Sprintf("发生错误：%v", msg.err),
			})
			m.followTail = true
			m.updateViewportContent(m.renderChat())
			return m, nil
		}

		m.state = msg.state
		if m.state.Context == nil {
			m.state.Context = map[string]interface{}{}
		}
		m.state.Context[agent.ConfirmEnabledContextKey] = m.opts.ConfirmTools

		m.updateViewportContent(m.renderChat())

		if awaiting, ok := m.state.Context[agent.ConfirmAwaitingContextKey].(bool); ok && awaiting {
			m.startConfirmPrompt()
			return m, nil
		}

		m.startStreamingFrom(msg.prevCount)
		if m.streaming {
			m.updateViewportContent(m.renderChat())
			return m, streamTick()
		}
		return m, nil

	case streamTickMsg:
		if !m.streaming {
			return m, nil
		}
		m.streamPos = min(len(m.streamFull), m.streamPos+32)
		m.overrideContent[m.streamIdx] = m.streamFull[:m.streamPos]
		m.updateViewportContent(m.renderChat())
		if m.streamPos >= len(m.streamFull) {
			m.streaming = false
		}
		if m.streaming {
			return m, streamTick()
		}
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit
		}

		if m.confirmVisible {
			switch msg.String() {
			case "left", "shift+tab":
				m.confirmIndex = (m.confirmIndex + 1) % 2
				return m, nil
			case "right", "tab":
				m.confirmIndex = (m.confirmIndex + 1) % 2
				return m, nil
			case "esc":
				m.confirmVisible = false
				m.state.Context[agent.ConfirmGrantedContextKey] = false
				m.state.UserQuery = "我拒绝执行工具操作，请给出替代方案。"
				m.state.Context[agent.ConfirmEnabledContextKey] = m.opts.ConfirmTools

				m.thinking = true
				prev := len(m.state.Messages)
				m.lastInvokePrevCount = prev
				return m, invokeBackend(m.ctx, m.backend, m.state, prev)
			case "enter":
				granted := m.confirmIndex == 0
				m.confirmVisible = false
				m.state.Context[agent.ConfirmGrantedContextKey] = granted
				if granted {
					m.state.UserQuery = ""
				} else {
					m.state.UserQuery = "我拒绝执行工具操作，请给出替代方案。"
				}
				m.state.Context[agent.ConfirmEnabledContextKey] = m.opts.ConfirmTools

				m.thinking = true
				m.followTail = true
				prev := len(m.state.Messages)
				m.lastInvokePrevCount = prev
				return m, invokeBackend(m.ctx, m.backend, m.state, prev)
			default:
				return m, nil
			}
		}

		switch msg.String() {
		case "pgup", "pageup":
			m.viewport.PageUp()
			m.followTail = false
			return m, nil
		case "pgdown", "pagedown":
			m.viewport.PageDown()
			if m.viewport.AtBottom() {
				m.followTail = true
			}
			return m, nil
		}

		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)

		if msg.String() == "enter" {
			text := strings.TrimSpace(m.input.Value())
			if text == "" {
				return m, cmd
			}
			switch strings.ToLower(text) {
			case "exit", "quit":
				return m, tea.Quit
			}

			m.state.Context[agent.ConfirmEnabledContextKey] = m.opts.ConfirmTools
			m.state.UserQuery = text
			m.state.Messages = append(m.state.Messages, schema.UserMessage(text))
			m.followTail = true
			m.updateViewportContent(m.renderChat())

			m.input.SetValue("")
			m.thinking = true
			prev := len(m.state.Messages)
			m.lastInvokePrevCount = prev
			return m, tea.Batch(cmd, invokeBackend(m.ctx, m.backend, m.state, prev))
		}

		return m, cmd
	}

	var cmd tea.Cmd
	m.input, cmd = m.input.Update(msg)
	return m, cmd
}

func (m chatModel) View() string {
	header := lipgloss.NewStyle().Bold(true).Render("CentAgent Chat")

	chat := m.viewport.View()

	var inputLine string
	if m.confirmVisible {
		inputLine = m.confirmView()
	} else {
		inputLine = m.inputView()
	}

	footer := m.footerView()

	return lipgloss.JoinVertical(lipgloss.Left, header, chat, inputLine, footer)
}

func (m chatModel) footerView() string {
	left := "Enter 发送 | PgUp/PgDn 滚动 | Ctrl+C 退出"
	right := ""
	if m.confirmVisible {
		right = "Tab/←/→ 切换  Enter 确认  Esc 取消"
	} else if m.thinking {
		right = m.spinner.View() + " Thinking..."
	}
	style := lipgloss.NewStyle().Width(m.width).Padding(0, 1)
	return style.Render(lipgloss.JoinHorizontal(lipgloss.Left, left, lipgloss.NewStyle().Width(max(0, m.width-lipgloss.Width(left)-lipgloss.Width(right)-2)).Render(""), right))
}

func (m chatModel) inputView() string {
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Width(max(1, m.input.Width+2)).
		Render(m.input.View())
	return box
}

func (m *chatModel) startConfirmPrompt() {
	title := "允许执行工具操作？"
	if len(m.state.Messages) > 0 {
		last := m.state.Messages[len(m.state.Messages)-1]
		if last != nil && last.Role == schema.Assistant && strings.TrimSpace(last.Content) != "" {
			title = strings.TrimSpace(last.Content)
		}
	}

	if idx := strings.Index(title, "是否允许"); idx >= 0 {
		title = strings.TrimSpace(title[:idx])
	}

	m.confirmTitle = title
	m.confirmVisible = true
	m.confirmIndex = 0
}

func (m chatModel) confirmView() string {
	title := m.confirmTitle
	if strings.TrimSpace(title) == "" {
		title = "允许执行工具操作？"
	}

	active := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(0, 2).
		Bold(true)
	inactive := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Padding(0, 2)

	leftBtn := inactive.Render("允许")
	rightBtn := inactive.Render("取消")
	if m.confirmIndex == 0 {
		leftBtn = active.Render("允许")
	} else {
		rightBtn = active.Render("取消")
	}

	buttons := lipgloss.JoinHorizontal(lipgloss.Left, leftBtn, " ", rightBtn)
	box := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		Padding(0, 1).
		Render(m.wrapToWidth(title, m.bubbleMaxContentWidth()) + "\n\n" + buttons)
	return box
}

func (m *chatModel) updateViewportContent(content string) {
	oldYOffset := m.viewport.YOffset
	m.viewport.SetContent(content)
	if m.followTail {
		m.viewport.GotoBottom()
		return
	}
	m.viewport.SetYOffset(oldYOffset)
}

func invokeBackend(ctx context.Context, backend ui.ChatBackend, state agent.AgentState, prevCount int) tea.Cmd {
	return func() tea.Msg {
		next, err := invokeBackendDiscardingStdIO(ctx, backend, state)
		return backendResultMsg{state: next, err: err, prevCount: prevCount}
	}
}

func invokeBackendDiscardingStdIO(ctx context.Context, backend ui.ChatBackend, state agent.AgentState) (agent.AgentState, error) {
	devNull, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		return backend.Invoke(ctx, state)
	}
	defer devNull.Close()

	stdioMu.Lock()
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	os.Stdout = devNull
	os.Stderr = devNull
	stdioMu.Unlock()

	next, invokeErr := backend.Invoke(ctx, state)

	stdioMu.Lock()
	os.Stdout = oldStdout
	os.Stderr = oldStderr
	stdioMu.Unlock()

	return next, invokeErr
}

func streamTick() tea.Cmd {
	return tea.Tick(45*time.Millisecond, func(time.Time) tea.Msg { return streamTickMsg{} })
}

func (m *chatModel) startStreamingFrom(prevCount int) {
	m.streaming = false
	m.streamFull = ""
	m.streamPos = 0
	m.streamIdx = -1

	if prevCount < 0 {
		prevCount = 0
	}
	for i := prevCount; i < len(m.state.Messages); i++ {
		msg := m.state.Messages[i]
		if msg == nil {
			continue
		}
		if msg.Role != schema.Assistant {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		m.streaming = true
		m.streamIdx = i
		m.streamFull = msg.Content
		m.streamPos = min(len(m.streamFull), 32)
		preview := m.streamFull[:m.streamPos]
		if strings.TrimSpace(preview) == "" {
			preview = "…"
		}
		m.overrideContent[i] = preview
		return
	}
}

func (m *chatModel) resetMarkdownRenderer() {
	if m.width <= 0 {
		return
	}
	contentWidth := m.bubbleMaxContentWidth()
	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(contentWidth),
	)
	if err == nil {
		m.renderer = r
	}
}

func (m chatModel) renderChat() string {
	if m.width <= 0 {
		m.width = 80
	}

	var b strings.Builder
	for i, msg := range m.state.Messages {
		if msg == nil {
			continue
		}
		if msg.Role == schema.System {
			continue
		}

		content := msg.Content
		if override, ok := m.overrideContent[i]; ok && (m.streaming && m.streamIdx == i) {
			content = override
		}
		content = strings.TrimRight(content, "\n")
		if msg.Role == schema.Assistant && strings.TrimSpace(content) == "" {
			continue
		}

		line := m.renderOneMessage(msg.Role, content)
		if line == "" {
			continue
		}
		b.WriteString(line)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m chatModel) bubbleMaxContentWidth() int {
	if m.width <= 0 {
		return 72
	}
	return max(20, m.width-8)
}

func (m chatModel) bubbleMinContentWidth() int {
	return 10
}

func (m chatModel) desiredContentWidth(s string) int {
	maxAllowed := m.bubbleMaxContentWidth()
	w := maxLineWidth(s)
	w = max(m.bubbleMinContentWidth(), w)
	w = min(maxAllowed, w)
	return w
}

func (m chatModel) wrapToWidth(s string, width int) string {
	if width <= 0 {
		return s
	}
	return lipgloss.NewStyle().Width(width).Render(s)
}

func maxLineWidth(s string) int {
	s = strings.TrimRight(s, "\n")
	if strings.TrimSpace(s) == "" {
		return 0
	}
	lines := strings.Split(s, "\n")
	maxW := 0
	for _, line := range lines {
		line = strings.TrimRight(line, " ")
		w := lipgloss.Width(line)
		if w > maxW {
			maxW = w
		}
	}
	return maxW
}

func (m chatModel) renderOneMessage(role schema.RoleType, content string) string {
	switch role {
	case schema.User:
		return m.renderUser(content)
	case schema.Assistant:
		return m.renderAssistant(content)
	case schema.Tool:
		return m.renderTool(content)
	default:
		return m.renderTool(content)
	}
}

func (m chatModel) renderAssistant(content string) string {
	md := content
	if m.renderer != nil && strings.TrimSpace(md) != "" {
		if rendered, err := m.renderer.Render(md); err == nil {
			md = strings.TrimRight(rendered, "\n")
		}
	}
	md = m.wrapToWidth(md, m.desiredContentWidth(md))
	bubble := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(0, 1).
		MaxWidth(max(20, m.width-4)).
		Render(md)
	return bubble
}

func (m chatModel) renderUser(content string) string {
	content = m.wrapToWidth(content, m.desiredContentWidth(content))
	bubble := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("205")).
		Padding(0, 1).
		MaxWidth(max(20, m.width-4)).
		Render(content)
	return lipgloss.NewStyle().Width(m.width).Align(lipgloss.Right).Render(bubble)
}

func (m chatModel) renderTool(content string) string {
	label := "TOOL"
	body := content
	if strings.TrimSpace(body) == "" {
		body = "(无输出)"
	}
	body = m.wrapToWidth(body, m.desiredContentWidth(body))
	bubble := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("240")).
		Foreground(lipgloss.Color("245")).
		Padding(0, 1).
		MaxWidth(max(20, m.width-4)).
		Render(label + "\n" + body)
	return bubble
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
