import sqlite3

conn = sqlite3.connect(r"D:\Tencentt\Tencent Files\1528760842\文件\MobileFile\nsy_chat_live-master\sqlite.db")
cursor = conn.cursor()

# Check table count and row sample
cursor.execute("SELECT name FROM sqlite_master WHERE type='table'")
tables = cursor.fetchall()
print("Tables:", tables)

# Check chat_messages columns
cursor.execute("PRAGMA table_info(chat_messages)")
cols = cursor.fetchall()
print("\nchat_messages columns:", [c[1] for c in cols])

# Check sample chat_messages
cursor.execute("SELECT id, user_id, display_name, chat_room_id, msg_type, send_time, Time_str FROM chat_messages LIMIT 5")
rows = cursor.fetchall()
print("\nSample chat_messages:")
for r in rows:
    print(r)

# Check DISTINCT dates query
cursor.execute("""
SELECT DISTINCT 
  CASE 
    WHEN Time_str != '' AND Time_str IS NOT NULL THEN substr(Time_str, 1, 10)
    WHEN send_time > 10000000000 THEN date(send_time/1000, 'unixepoch', 'localtime')
    WHEN send_time > 0 THEN date(send_time, 'unixepoch', 'localtime')
    ELSE ''
  END as d
FROM chat_messages
WHERE d != ''
ORDER BY d ASC
""")
dates = cursor.fetchall()
print(f"\nTotal distinct dates in DB: {len(dates)}")
print("Sample dates:", dates[:10], "...", dates[-5:] if len(dates) > 5 else "")

# Check distinct chat rooms
cursor.execute("SELECT id, user_id, unique_id, display_name, chat_room_id FROM chat_rooms")
rooms = cursor.fetchall()
print("\nChat rooms in DB:")
for rm in rooms:
    # count messages for this room
    cursor.execute("SELECT count(*) FROM chat_messages WHERE user_id = ? OR chat_room_id = ? OR display_name = ?", (rm[1], rm[4], rm[3]))
    cnt = cursor.fetchone()[0]
    cursor.execute("SELECT id, msg_type, content, send_time, Time_str FROM chat_messages WHERE user_id = ? OR chat_room_id = ? OR display_name = ? ORDER BY id DESC LIMIT 1", (rm[1], rm[4], rm[3]))
    last_msg = cursor.fetchone()
    print(f"Room: {rm[3]} (uid={rm[1]}, room_id={rm[4]}), msg count: {cnt}, last msg: {last_msg}")

conn.close()
