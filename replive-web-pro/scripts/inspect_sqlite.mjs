import { DatabaseSync } from "node:sqlite";

try {
  const db = new DatabaseSync("D:/Tencentt/Tencent Files/1528760842/文件/MobileFile/nsy_chat_live-master/sqlite.db");
  console.log("Successfully connected to sqlite.db with node:sqlite!");
  
  // Inspect chat_rooms
  const rooms = db.prepare("SELECT id, user_id, unique_id, display_name, chat_room_id FROM chat_rooms").all();
  console.log("\nRooms in DB:", rooms);

  for (const r of rooms) {
    // Count messages
    const countRow = db.prepare("SELECT count(*) as cnt FROM chat_messages WHERE user_id = ? OR chat_room_id = ?").get(r.user_id, r.chat_room_id);
    console.log(`\nRoom ${r.display_name} (${r.chat_room_id}): msg count = ${countRow.cnt}`);

    // Last message
    const lastMsg = db.prepare("SELECT id, user_id, display_name, msg_type, content, send_time, Time_str FROM chat_messages WHERE user_id = ? OR chat_room_id = ? ORDER BY id DESC LIMIT 1").get(r.user_id, r.chat_room_id);
    console.log("Last message:", lastMsg);

    // Available dates
    const dates = db.prepare(`
      SELECT DISTINCT 
        CASE 
          WHEN Time_str != '' AND Time_str IS NOT NULL THEN substr(Time_str, 1, 10)
          WHEN send_time > 10000000000 THEN date(send_time/1000, 'unixepoch', 'localtime')
          WHEN send_time > 0 THEN date(send_time, 'unixepoch', 'localtime')
          ELSE ''
        END as d
      FROM chat_messages
      WHERE (user_id = ? OR chat_room_id = ?) AND d != ''
      ORDER BY d ASC
    `).all(r.user_id, r.chat_room_id);
    console.log(`Available dates (${dates.length} days):`, dates.map(x => x.d));
  }

} catch (err) {
  console.error("Error inspecting SQLite:", err);
}
