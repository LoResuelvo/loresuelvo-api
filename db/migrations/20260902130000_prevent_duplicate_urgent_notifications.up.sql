DELETE FROM notifications older
USING notifications newer
WHERE older.type = 'work_order_close_to_scheduled_time'
  AND older.resource_type = 'work_order'
  AND newer.type = older.type
  AND newer.resource_type = older.resource_type
  AND newer.user_id = older.user_id
  AND newer.resource_id = older.resource_id
  AND newer.id > older.id;

CREATE UNIQUE INDEX notifications_urgent_work_order_unique_idx
    ON notifications (user_id, type, resource_id)
    WHERE type = 'work_order_close_to_scheduled_time'
      AND resource_type = 'work_order';
