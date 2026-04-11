package background

import (
	"encoding/json"

	"dev.helix.agent/internal/models"
	extractedbackground "digital.vasic.background"
	extractedmodels "digital.vasic.models"
)

// convertToInternalTask converts extracted BackgroundTask to internal BackgroundTask
func convertToInternalTask(extracted *extractedmodels.BackgroundTask) *models.BackgroundTask {
	if extracted == nil {
		return nil
	}
	// Use JSON marshaling/unmarshaling for conversion since structs are identical
	data, err := json.Marshal(extracted)
	if err != nil {
		return nil
	}
	var internal models.BackgroundTask
	if err := json.Unmarshal(data, &internal); err != nil {
		return nil
	}
	return &internal
}

// convertToExtractedTask converts internal BackgroundTask to extracted BackgroundTask
func convertToExtractedTask(internal *models.BackgroundTask) *extractedmodels.BackgroundTask {
	if internal == nil {
		return nil
	}
	data, err := json.Marshal(internal)
	if err != nil {
		return nil
	}
	var extracted extractedmodels.BackgroundTask
	if err := json.Unmarshal(data, &extracted); err != nil {
		return nil
	}
	return &extracted
}

// convertToInternalTasks converts slice of extracted BackgroundTask to slice of internal BackgroundTask
func convertToInternalTasks(extracted []*extractedmodels.BackgroundTask) []*models.BackgroundTask {
	if extracted == nil {
		return nil
	}
	result := make([]*models.BackgroundTask, len(extracted))
	for i, task := range extracted {
		result[i] = convertToInternalTask(task)
	}
	return result
}

// convertToExtractedTasks converts slice of internal BackgroundTask to slice of extracted BackgroundTask
func convertToExtractedTasks(internal []*models.BackgroundTask) []*extractedmodels.BackgroundTask {
	if internal == nil {
		return nil
	}
	result := make([]*extractedmodels.BackgroundTask, len(internal))
	for i, task := range internal {
		result[i] = convertToExtractedTask(task)
	}
	return result
}

// convertTaskPriority converts internal TaskPriority to extracted TaskPriority
func convertTaskPriority(priority models.TaskPriority) extractedmodels.TaskPriority {
	return extractedmodels.TaskPriority(priority)
}

// convertToInternalResourceSnapshot converts extracted ResourceSnapshot to internal ResourceSnapshot
func convertToInternalResourceSnapshot(extracted *extractedmodels.ResourceSnapshot) *models.ResourceSnapshot {
	if extracted == nil {
		return nil
	}
	data, err := json.Marshal(extracted)
	if err != nil {
		return nil
	}
	var internal models.ResourceSnapshot
	if err := json.Unmarshal(data, &internal); err != nil {
		return nil
	}
	return &internal
}

// convertToExtractedResourceSnapshot converts internal ResourceSnapshot to extracted ResourceSnapshot
func convertToExtractedResourceSnapshot(internal *models.ResourceSnapshot) *extractedmodels.ResourceSnapshot {
	if internal == nil {
		return nil
	}
	data, err := json.Marshal(internal)
	if err != nil {
		return nil
	}
	var extracted extractedmodels.ResourceSnapshot
	if err := json.Unmarshal(data, &extracted); err != nil {
		return nil
	}
	return &extracted
}

// convertToInternalSystemResources converts extracted SystemResources to internal SystemResources
func convertToInternalSystemResources(extracted *extractedbackground.SystemResources) *SystemResources {
	if extracted == nil {
		return nil
	}
	data, err := json.Marshal(extracted)
	if err != nil {
		return nil
	}
	var internal SystemResources
	if err := json.Unmarshal(data, &internal); err != nil {
		return nil
	}
	return &internal
}

// convertToExtractedSystemResources converts internal SystemResources to extracted SystemResources
func convertToExtractedSystemResources(internal *SystemResources) *extractedbackground.SystemResources {
	if internal == nil {
		return nil
	}
	data, err := json.Marshal(internal)
	if err != nil {
		return nil
	}
	var extracted extractedbackground.SystemResources
	if err := json.Unmarshal(data, &extracted); err != nil {
		return nil
	}
	return &extracted
}

// convertToInternalResourceRequirements converts extracted ResourceRequirements to internal ResourceRequirements
func convertToInternalResourceRequirements(extracted extractedbackground.ResourceRequirements) ResourceRequirements {
	return ResourceRequirements{
		CPUCores: extracted.CPUCores,
		MemoryMB: extracted.MemoryMB,
		DiskMB:   extracted.DiskMB,
		GPUCount: extracted.GPUCount,
		Priority: models.TaskPriority(extracted.Priority),
	}
}

// convertToExtractedResourceRequirements converts internal ResourceRequirements to extracted ResourceRequirements
func convertToExtractedResourceRequirements(internal ResourceRequirements) extractedbackground.ResourceRequirements {
	return extractedbackground.ResourceRequirements{
		CPUCores: internal.CPUCores,
		MemoryMB: internal.MemoryMB,
		DiskMB:   internal.DiskMB,
		GPUCount: internal.GPUCount,
		Priority: extractedmodels.TaskPriority(internal.Priority),
	}
}

// convertToInternalStuckAnalysis converts extracted StuckAnalysis to internal StuckAnalysis
func convertToInternalStuckAnalysis(extracted *extractedbackground.StuckAnalysis) *StuckAnalysis {
	if extracted == nil {
		return nil
	}
	// Manual field copying to avoid JSON omitempty issues with empty slices
	internal := &StuckAnalysis{
		IsStuck: extracted.IsStuck,
		Reason:  extracted.Reason,
		HeartbeatStatus: HeartbeatStatus{
			LastHeartbeat:      extracted.HeartbeatStatus.LastHeartbeat,
			TimeSinceHeartbeat: extracted.HeartbeatStatus.TimeSinceHeartbeat,
			Threshold:          extracted.HeartbeatStatus.Threshold,
			IsStale:            extracted.HeartbeatStatus.IsStale,
		},
		ResourceStatus: ResourceStatus{
			CPUPercent:    extracted.ResourceStatus.CPUPercent,
			MemoryPercent: extracted.ResourceStatus.MemoryPercent,
			MemoryBytes:   extracted.ResourceStatus.MemoryBytes,
			OpenFDs:       extracted.ResourceStatus.OpenFDs,
			ThreadCount:   extracted.ResourceStatus.ThreadCount,
			IsExhausted:   extracted.ResourceStatus.IsExhausted,
		},
		ActivityStatus: ActivityStatus{
			HasCPUActivity: extracted.ActivityStatus.HasCPUActivity,
			HasIOActivity:  extracted.ActivityStatus.HasIOActivity,
			HasNetActivity: extracted.ActivityStatus.HasNetActivity,
			IOReadBytes:    extracted.ActivityStatus.IOReadBytes,
			IOWriteBytes:   extracted.ActivityStatus.IOWriteBytes,
			NetConnections: extracted.ActivityStatus.NetConnections,
		},
	}
	// Copy recommendations slice (may be nil or empty)
	if extracted.Recommendations != nil {
		internal.Recommendations = make([]string, len(extracted.Recommendations))
		copy(internal.Recommendations, extracted.Recommendations)
	} else {
		internal.Recommendations = []string{} // Ensure non-nil empty slice
	}
	return internal
}

// convertToInternalWorkerStatus converts extracted WorkerStatus to internal WorkerStatus
func convertToInternalWorkerStatus(extracted *extractedbackground.WorkerStatus) *WorkerStatus {
	if extracted == nil {
		return nil
	}
	return &WorkerStatus{
		ID:              extracted.ID,
		Status:          extracted.Status,
		CurrentTask:     convertToInternalTask(extracted.CurrentTask),
		StartedAt:       extracted.StartedAt,
		LastActivity:    extracted.LastActivity,
		TasksCompleted:  extracted.TasksCompleted,
		TasksFailed:     extracted.TasksFailed,
		AvgTaskDuration: extracted.AvgTaskDuration,
	}
}
