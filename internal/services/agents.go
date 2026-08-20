package services

import (
	"fmt"

	"github.com/mustafanass/uruflow/internal/models"
	"github.com/mustafanass/uruflow/internal/storage"
)

func DeleteAgent(store storage.Store, revoke func(string), agentID string) error {
	agent, err := store.GetAgent(agentID)
	if err != nil {
		return err
	}
	projects, err := store.ListProjects()
	if err != nil {
		return err
	}
	for _, project := range projects {
		if project.Builder == agentID || containsAgent(project.Runners, agentID) {
			return fmt.Errorf("agent %s is still used by project %s", agent.Name, project.Name)
		}
	}
	if agent.Status == models.AgentOnline && revoke == nil {
		return fmt.Errorf("agent %s is online; remove it from the running server interface", agent.Name)
	}
	if revoke != nil {
		revoke(agentID)
	}
	return store.DeleteAgent(agentID)
}

func containsAgent(agentIDs []string, wanted string) bool {
	for _, agentID := range agentIDs {
		if agentID == wanted {
			return true
		}
	}
	return false
}
