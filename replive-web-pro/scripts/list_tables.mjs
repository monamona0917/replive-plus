import { DatabaseSync } from "node:sqlite";

const db = new DatabaseSync("D:/Tencentt/Tencent Files/1528760842/文件/MobileFile/nsy_chat_live-master/sqlite.db");
const tables = db.prepare("SELECT name FROM sqlite_master WHERE type='table'").all();
console.log("All tables in sqlite.db:", tables);
