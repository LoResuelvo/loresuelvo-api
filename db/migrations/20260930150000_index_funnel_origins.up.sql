-- Period selection operates on origins, not downstream milestones.
CREATE INDEX problem_assessments_professional_created_on_idx
    ON problem_assessments (created_on)
    WHERE outcome = 'professional_required';

CREATE INDEX job_requests_manual_created_on_idx
    ON job_requests (created_on)
    WHERE source_assessment_id IS NULL;
