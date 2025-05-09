package actions

import (
	"context"
	"fmt"

	"github.com/mudler/LocalAGI/core/state"
	"github.com/mudler/LocalAGI/core/types"
	"github.com/sashabaranov/go-openai"
	"github.com/sashabaranov/go-openai/jsonschema"
)

func NewCallAgent(config map[string]string, agentName string, pool *state.AgentPoolInternalAPI) *CallAgentAction {
	return &CallAgentAction{
		pool:   pool,
		myName: agentName,
	}
}

type CallAgentAction struct {
	pool   *state.AgentPoolInternalAPI
	myName string
}

func (a *CallAgentAction) Run(ctx context.Context, params types.ActionParams) (types.ActionResult, error) {
	result := struct {
		AgentName string `json:"agent_name"`
		Message   string `json:"message"`
	}{}
	err := params.Unmarshal(&result)
	if err != nil {
		fmt.Printf("error: %v", err)

		return types.ActionResult{}, err
	}

	ag := a.pool.GetAgent(result.AgentName)
	if ag == nil {
		return types.ActionResult{}, fmt.Errorf("agent '%s' not found", result.AgentName)
	}

	resp := ag.Ask(
		types.WithConversationHistory(
			[]openai.ChatCompletionMessage{
				{
					Role:    "user",
					Content: result.Message,
				},
			},
		),
	)
	if resp.Error != nil {
		return types.ActionResult{}, err
	}

	metadata := make(map[string]interface{})

	for _, s := range resp.State {
		for k, v := range s.Metadata {
			if existingValue, ok := metadata[k]; ok {
				switch existingValue := existingValue.(type) {
				case []string:
					switch v := v.(type) {
					case []string:
						metadata[k] = append(existingValue, v...)
					case string:
						metadata[k] = append(existingValue, v)
					}
				case string:
					switch v := v.(type) {
					case []string:
						metadata[k] = append([]string{existingValue}, v...)
					case string:
						metadata[k] = []string{existingValue, v}
					}
				}
			} else {
				metadata[k] = v
			}
		}
	}

	return types.ActionResult{Result: resp.Response, Metadata: metadata}, nil
}

func (a *CallAgentAction) Definition() types.ActionDefinition {
	allAgents := a.pool.AllAgents()

	agents := []string{}

	for _, ag := range allAgents {
		if ag != a.myName {
			agents = append(agents, ag)
		}
	}

	description := "Use this tool to call another agent. Available agents and their roles are:"

	for _, agent := range agents {
		agentConfig := a.pool.GetConfig(agent)
		if agentConfig == nil {
			continue
		}
		description += fmt.Sprintf("\n\t- %s: %s", agent, agentConfig.Description)
	}

	return types.ActionDefinition{
		Name:        "call_agent",
		Description: description,
		Properties: map[string]jsonschema.Definition{
			"agent_name": {
				Type:        jsonschema.String,
				Description: "The name of the agent to call.",
				Enum:        agents,
			},
			"message": {
				Type:        jsonschema.String,
				Description: "The message to send to the agent.",
			},
		},
		Required: []string{"agent_name", "message"},
	}
}

func (a *CallAgentAction) Plannable() bool {
	return true
}
