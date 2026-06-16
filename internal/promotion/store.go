package promotion

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// t1 builds a Table-1 per-rank point map (current-rank key -> points). Columns
// follow جدول رقم (١): promotion to مدرس / أستاذ مساعد / أستاذ.
func t1(toLecturer, toAsstProf, toProf float64) map[string]float64 {
	return map[string]float64{
		"assistant_lecturer":  toLecturer, // مدرس مساعد -> مدرس
		"lecturer":            toAsstProf, // مدرس -> أستاذ مساعد
		"assistant_professor": toProf,     // أستاذ مساعد -> أستاذ
	}
}

// t2 builds a rank-independent point map for Table-2 items.
func t2(p float64) map[string]float64 { return map[string]float64{"*": p} }

// DefaultRules encodes تعليمات الترقيات العلمية رقم (١٠) لسنة ٢٠٢٥ (الجدولان ١ و٢
// والمواد ١–٣). Values are official but editable via data/promotion.yaml.
// Table 2 here covers the most common activities; the admin can append the rest
// of the annex. النتيجة استرشادية والقرار النهائي للجنة الترقيات.
func DefaultRules() *Rules {
	return &Rules{
		Ranks: []Rank{
			{Key: "assistant_lecturer", Label: "مدرس مساعد", NextLabel: "مدرس",
				RequiredTotal: 70, RequiredTable1: 46, RequiredTable2: 24, MinServiceYears: 3},
			{Key: "lecturer", Label: "مدرس", NextLabel: "أستاذ مساعد",
				RequiredTotal: 80, RequiredTable1: 52, RequiredTable2: 28, MinServiceYears: 4},
			{Key: "assistant_professor", Label: "أستاذ مساعد", NextLabel: "أستاذ",
				RequiredTotal: 90, RequiredTable1: 59, RequiredTable2: 31, MinServiceYears: 6},
		},
		Activities: []Activity{
			// --- جدول رقم (١): البحوث والنتاج العلمي ---
			// مجلات IF/CS عالمية (مطبوعة/الكترونية).
			{Key: "if_first", Label: "بحث IF/CS — الباحث الأول", Table: 1, Points: t1(30, 20, 20)},
			{Key: "if_second", Label: "بحث IF/CS — الباحث الثاني", Table: 1, Points: t1(24, 16, 16)},
			{Key: "if_third", Label: "بحث IF/CS — الباحث الثالث", Table: 1, Points: t1(21, 14, 14)},
			{Key: "if_fourth", Label: "بحث IF/CS — الباحث الرابع", Table: 1, Points: t1(15, 10, 10)},
			{Key: "if_fifth", Label: "بحث IF/CS — الباحث الخامس", Table: 1, Points: t1(12, 8, 8)},
			{Key: "if_sixthplus", Label: "بحث IF/CS — ما بعد الخامس", Table: 1, Points: t1(6, 4, 4)},
			// مجلات عراقية معتمدة من الوزارة.
			{Key: "iq_first", Label: "بحث عراقي معتمد — الباحث الأول", Table: 1, Points: t1(20, 15, 10)},
			{Key: "iq_second", Label: "بحث عراقي معتمد — الباحث الثاني", Table: 1, Points: t1(16, 12, 8)},
			{Key: "iq_third", Label: "بحث عراقي معتمد — الباحث الثالث", Table: 1, Points: t1(14, 10.5, 7)},
			{Key: "iq_fourth", Label: "بحث عراقي معتمد — الباحث الرابع", Table: 1, Points: t1(10, 7.5, 5)},
			{Key: "iq_fifth", Label: "بحث عراقي معتمد — الباحث الخامس", Table: 1, Points: t1(8, 6, 4)},
			{Key: "iq_sixthplus", Label: "بحث عراقي معتمد — ما بعد الخامس", Table: 1, Points: t1(4, 3, 2)},

			// --- جدول رقم (٢): النشاطات وخدمة المجتمع ---
			{Key: "book_local_solo", Label: "كتاب محلي — منفرد", Table: 2, Points: t2(8)},
			{Key: "book_local_two", Label: "كتاب محلي — مشترك (٢)", Table: 2, Points: t2(5)},
			{Key: "book_local_multi", Label: "كتاب محلي — مشترك (٣+)", Table: 2, Points: t2(3)},
			{Key: "book_intl_solo", Label: "كتاب عالمي — منفرد", Table: 2, Points: t2(10)},
			{Key: "book_intl_two", Label: "كتاب عالمي — مشترك (٢)", Table: 2, Points: t2(8)},
			{Key: "book_intl_multi", Label: "كتاب عالمي — مشترك (٣+)", Table: 2, Points: t2(5)},
			{Key: "book_chapter", Label: "فصل كتاب (Book Chapter)", Table: 2, Points: t2(5), Cap: 6},
			{Key: "conference_paper", Label: "بحث منشور في مؤتمر علمي", Table: 2, Points: t2(5)},
			{Key: "community_study_in", Label: "دراسة لمشكلة مجتمعية — بالتخصص", Table: 2, Points: t2(10), Cap: 10},
			{Key: "community_study_out", Label: "دراسة لمشكلة مجتمعية — خارج التخصص", Table: 2, Points: t2(5), Cap: 5},
			{Key: "citations10", Label: "استشهادات (لكل ١٠)", Table: 2, Points: t2(1), Cap: 5},
			{Key: "review_article", Label: "مقال Review / Case Study", Table: 2, Points: t2(5), Cap: 10},
			{Key: "popular_article", Label: "مقال علمي بمجلة عامة", Table: 2, Points: t2(2), Cap: 5},
			{Key: "patent_intl", Label: "براءة اختراع دولية", Table: 2, Points: t2(25), Cap: 25},
			{Key: "patent_local", Label: "براءة اختراع محلية", Table: 2, Points: t2(5), Cap: 5},
			{Key: "award_intl", Label: "وسام/ميدالية دولية", Table: 2, Points: t2(10)},
			{Key: "award_local", Label: "وسام/ميدالية محلية", Table: 2, Points: t2(3)},
			{Key: "exam_committee", Label: "لجنة امتحانية", Table: 2, Points: t2(1), Cap: 5},
			{Key: "thesis_committee", Label: "لجنة مناقشة (ماجستير/دكتوراه)", Table: 2, Points: t2(1), Cap: 5},
			{Key: "scientific_committee", Label: "لجنة علمية/ترقيات", Table: 2, Points: t2(1), Cap: 5},
			{Key: "conf_committee", Label: "لجنة مؤتمر", Table: 2, Points: t2(1), Cap: 5},
			{Key: "ministerial_committee", Label: "لجنة وزارية", Table: 2, Points: t2(1), Cap: 5},
			{Key: "union_academics", Label: "عضوية نقابة الأكاديميين", Table: 2, Points: t2(3)},
			{Key: "union_professional", Label: "عضوية نقابة مهنية/جمعية علمية", Table: 2, Points: t2(1), Cap: 6},
			{Key: "supervise_master", Label: "إشراف ماجستير/دبلوم عالٍ", Table: 2, Points: t2(2), Cap: 4},
			{Key: "supervise_phd", Label: "إشراف دكتوراه/بورد", Table: 2, Points: t2(3), Cap: 6},
			{Key: "editor_chief", Label: "رئيس/مدير تحرير مجلة معتمدة", Table: 2, Points: t2(10)},
			{Key: "editor_member", Label: "عضو هيئة تحرير مجلة معتمدة", Table: 2, Points: t2(5)},
			{Key: "h_index_excess", Label: "مؤشر هيرتش (لكل قيمة فوق المطلوب)", Table: 2, Points: t2(1), Cap: 10},
		},
	}
}

type fileShape struct {
	Activities []Activity `yaml:"activities"`
	Ranks      []Rank     `yaml:"ranks"`
}

// Load reads data/promotion.yaml, falling back to DefaultRules when absent or
// empty so the bot still starts on a fresh checkout.
func Load(dataDir string) (*Rules, error) {
	path := filepath.Join(dataDir, "promotion.yaml")
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return DefaultRules(), nil
		}
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var doc fileShape
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(doc.Ranks) == 0 || len(doc.Activities) == 0 {
		return DefaultRules(), nil
	}
	return &Rules{Activities: doc.Activities, Ranks: doc.Ranks}, nil
}
