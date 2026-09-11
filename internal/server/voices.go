package server

// BuiltinVoices 返回内置音色表（字段 id/gender/category）。
// 音色 ID 取自火山引擎大模型语音合成公开音色列表（见 task-9-report）；
// 后续里程碑接入在线音色列表接口（ListSpeakers）后由真实列表替换内置表。
func BuiltinVoices() []map[string]string {
	return []map[string]string{
		{"id": "zh_female_cancan_mars_bigtts", "gender": "女", "category": "通用"},
		{"id": "zh_female_mizaitongxue_v2_saturn_bigtts", "gender": "女", "category": "童声"},
		{"id": "zh_female_shuangkuaisisi_moon_bigtts", "gender": "女", "category": "通用"},
		{"id": "zh_female_linjianvhai_moon_bigtts", "gender": "女", "category": "通用"},
		{"id": "zh_female_wanwanxiaohe_moon_bigtts", "gender": "女", "category": "方言"},
		{"id": "zh_male_dayixiansheng_v2_saturn_bigtts", "gender": "男", "category": "通用"},
		{"id": "zh_male_beijingxiaoye_v2_mars_bigtts", "gender": "男", "category": "方言"},
		{"id": "zh_male_shaonianzixin_moon_bigtts", "gender": "男", "category": "通用"},
		{"id": "zh_male_yangguangqingnian_moon_bigtts", "gender": "男", "category": "通用"},
		{"id": "zh_male_dongfanghaoran_moon_bigtts", "gender": "男", "category": "通用"},
	}
}
