import fs from "node:fs/promises";
import { FileBlob, SpreadsheetFile, Workbook } from "@oai/artifact-tool";

function arg(name, fallback = undefined) {
  const index = process.argv.indexOf(`--${name}`);
  if (index >= 0 && process.argv[index + 1]) return process.argv[index + 1];
  if (fallback !== undefined) return fallback;
  throw new Error(`Missing required --${name} argument`);
}

const analysisPath = arg("analysis");
const sourceXlsx = arg("source-xlsx");
const sourceCsv = arg("source-csv");
const outputPath = arg("output");
const previewDir = arg("preview-dir", outputPath.slice(0, outputPath.lastIndexOf("/")));

const analysisData = JSON.parse(await fs.readFile(analysisPath, "utf8"));
if (analysisData.schema !== "dice-throne-character-analysis-v1") {
  throw new Error("Analysis JSON must use dice-throne-character-analysis-v1");
}
const characterName = analysisData.character.name;
const characterId = analysisData.character.id;
const valueLabel = analysisData.character.value_label ?? "listed value";
const analysisSheetName = `${characterName} Roll Analysis`.slice(0, 31);
const overallSheetName = `${characterName} Overall Value`.slice(0, 31);
const safeTablePrefix = characterId.replace(/[^a-zA-Z0-9]/g, "") || "Character";
const sourceBlob = await FileBlob.load(sourceXlsx);
const workbook = await SpreadsheetFile.importXlsx(sourceBlob);
const csvText = await fs.readFile(sourceCsv, "utf8");
const csvWorkbook = await Workbook.fromCSV(csvText, { sheetName: "Reference" });
const csvValues = csvWorkbook.worksheets.getItem("Reference").getUsedRange().values;

const reference = workbook.worksheets.getItem("Reference");
for (const table of reference.tables.items) table.delete();
reference.getRange(`A1:P${csvValues.length}`).values = csvValues;
const referenceTable = reference.tables.add(
  `A1:P${csvValues.length}`,
  true,
  "DiceThroneReferenceTable",
);
referenceTable.style = "TableStyleLight1";
reference.showGridLines = false;
reference.freezePanes.freezeRows(1);
reference.getRange("A1:P1").format = {
  fill: "#29263D",
  font: { bold: true, color: "#FFFFFF", typeface: "Aptos Display", fontSize: 10 },
  wrapText: true,
  borders: { bottom: { style: "medium", color: "#8C79B8" } },
};
reference.getRange(`A2:P${csvValues.length}`).format = {
  fill: "#F7F5FB",
  font: { color: "#29263D", typeface: "Aptos", fontSize: 9 },
  verticalAlignment: "top",
  wrapText: true,
  borders: { insideHorizontal: { style: "thin", color: "#DED9EA" } },
};
reference.getRange(`A2:P${csvValues.length}`).format.rowHeight = 36;
const referenceWidths = [
  ["A:A", 17], ["B:B", 18], ["C:C", 25], ["D:D", 24],
  ["E:E", 36], ["F:F", 11], ["G:G", 10], ["H:H", 15],
  ["I:I", 76], ["J:J", 18], ["K:K", 15], ["L:L", 14],
  ["M:M", 16], ["N:N", 18], ["O:O", 21], ["P:P", 19],
];
for (const [column, width] of referenceWidths) {
  reference.getRange(column).format.columnWidth = width;
}

const analysis = workbook.worksheets.add(analysisSheetName);
analysis.showGridLines = false;
analysis.freezePanes.freezeRows(8);
analysis.freezePanes.freezeColumns(2);
analysis.getRange("A1:N1").merge();
analysis.getRange("A1").values = [[`${characterName} Ability Roll Analysis`]];
analysis.getRange("A1:N1").format = {
  fill: "#29263D",
  font: { bold: true, color: "#FFFFFF", typeface: "Aptos Display", fontSize: 18 },
  verticalAlignment: "center",
};
analysis.getRange("A1:N1").format.rowHeight = 32;
analysis.getRange("A2:N2").merge();
analysis.getRange("A2").values = [[
  "Fresh 5d6 roll with optimal keeps and up to two rerolls. Odds are exact; recommendations are neutral-start guidance and should pivot after seeing the opening dice.",
]];
analysis.getRange("A2:N2").format = {
  fill: "#EDE9F6", font: { color: "#433B61", italic: true, fontSize: 10 },
  wrapText: true, verticalAlignment: "center",
};
analysis.getRange("A2:N2").format.rowHeight = 30;

const abilityFirstRow = 9;
const abilityLastRow = abilityFirstRow + analysisData.abilities.length - 1;
const mostReliable = [...analysisData.abilities].sort((a, b) => b.two_reroll_probability - a.two_reroll_probability)[0];
const bestOpening = [...analysisData.abilities].sort((a, b) => b.first_roll_probability - a.first_roll_probability)[0];
const hardest = [...analysisData.abilities].sort((a, b) => a.two_reroll_probability - b.two_reroll_probability)[0];
const bestListed = [...analysisData.abilities].sort((a, b) => b.expected_listed_value - a.expected_listed_value)[0];
const abilityRowFor = (ability) => abilityFirstRow + analysisData.abilities.findIndex((row) => row.ability === ability.ability);
const cards = [
  ["A4:C4", "Most reliable target", "A5", mostReliable.ability, "B5", `=H${abilityRowFor(mostReliable)}`, "0.00%"],
  ["D4:F4", "Best opening-roll target", "D5", bestOpening.ability, "E5", `=F${abilityRowFor(bestOpening)}`, "0.00%"],
  ["G4:I4", "Hardest neutral target", "G5", hardest.ability, "H5", `=H${abilityRowFor(hardest)}`, "0.00%"],
  ["J4:L4", `Best target-only ${valueLabel}`, "J5", bestListed.ability, "K5", `=J${abilityRowFor(bestListed)}`, "0.00"],
];
for (const [headerRange, header, labelCell, label, valueCell, formula, numberFormat] of cards) {
  analysis.getRange(headerRange).merge();
  analysis.getRange(headerRange.split(":")[0]).values = [[header]];
  analysis.getRange(headerRange).format = {
    fill: "#8C79B8", font: { bold: true, color: "#FFFFFF", fontSize: 10 },
    horizontalAlignment: "center",
  };
  analysis.getRange(labelCell).values = [[label]];
  analysis.getRange(valueCell).formulas = [[formula]];
  analysis.getRange(valueCell).format.numberFormat = numberFormat;
  analysis.getRange(`${labelCell}:${valueCell}`).format = {
    fill: "#F7F5FB", font: { bold: true, color: "#29263D" },
    horizontalAlignment: "center", borders: { preset: "outside", style: "thin", color: "#CFC7DF" },
  };
}

analysis.getRange("A7:N7").merge();
analysis.getRange("A7").values = [["Ability difficulty and neutral-start recommendation"]];
analysis.getRange("A7:N7").format = { fill: "#433B61", font: { bold: true, color: "#FFFFFF", fontSize: 11 } };
analysis.getRange("A8:N8").values = [[
  "Rank", "Ability", "Category", "Activation", "Listed result", "First roll", "1 reroll",
  "2 rerolls", "Difficulty", `Target-only expected ${valueLabel}`, "Value type", "Recommendation",
  "CSV row", "Source rules text",
]];
const abilityRows = analysisData.abilities.map((row) => [
  null, row.ability, row.category, row.requirement, row.effect, row.first_roll_probability,
  row.one_reroll_probability, row.two_reroll_probability, row.difficulty,
  row.expected_listed_value, row.value_kind, row.recommendation, row.source_csv_row,
  row.source_rules_text,
]);
analysis.getRange(`A${abilityFirstRow}:N${abilityLastRow}`).values = abilityRows;
for (let row = abilityFirstRow; row <= abilityLastRow; row += 1) {
  analysis.getRange(`A${row}`).formulas = [[`=COUNTIF($H$${abilityFirstRow}:$H$${abilityLastRow},">"&H${row})+1`]];
  analysis.getRange(`A${row}:N${row}`).format.fill = row % 2 === 1 ? "#F7F5FB" : "#FFFFFF";
}
analysis.getRange("A8:N8").format = {
  fill: "#433B61", font: { bold: true, color: "#FFFFFF", fontSize: 9 },
  wrapText: true, verticalAlignment: "center",
};
analysis.getRange(`F${abilityFirstRow}:H${abilityLastRow}`).format.numberFormat = "0.00%";
analysis.getRange(`J${abilityFirstRow}:J${abilityLastRow}`).format.numberFormat = "0.00";
analysis.getRange(`A${abilityFirstRow}:A${abilityLastRow}`).format.numberFormat = "0";
analysis.getRange(`M${abilityFirstRow}:M${abilityLastRow}`).format.numberFormat = "0";
analysis.getRange(`A8:N${abilityLastRow}`).format.verticalAlignment = "top";
analysis.getRange(`D${abilityFirstRow}:E${abilityLastRow}`).format.wrapText = true;
analysis.getRange(`L${abilityFirstRow}:N${abilityLastRow}`).format.wrapText = true;
analysis.getRange(`A${abilityFirstRow}:N${abilityLastRow}`).format.rowHeight = 52;

const tierSectionRow = abilityLastRow + 3;
const tierHeaderRow = tierSectionRow + 1;
const tierFirstRow = tierSectionRow + 2;
const tierLastRow = Math.max(tierFirstRow, tierFirstRow + analysisData.tier_probabilities.length - 1);
analysis.getRange(`A${tierSectionRow}:E${tierSectionRow}`).merge();
analysis.getRange(`A${tierSectionRow}`).values = [["Configured tier thresholds"]];
analysis.getRange(`A${tierSectionRow}:E${tierSectionRow}`).format = { fill: "#433B61", font: { bold: true, color: "#FFFFFF", fontSize: 11 } };
analysis.getRange(`A${tierHeaderRow}:E${tierHeaderRow}`).values = [["Tier target", "First roll", "1 reroll", "2 rerolls", "Interpretation"]];
analysis.getRange(`A${tierHeaderRow}:E${tierHeaderRow}`).format = { fill: "#433B61", font: { bold: true, color: "#FFFFFF", fontSize: 9 } };
if (analysisData.tier_probabilities.length) {
  const tierRows = analysisData.tier_probabilities.map((row) => [
    row.name, row.first_roll_probability, row.one_reroll_probability,
    row.two_reroll_probability, row.note,
  ]);
  analysis.getRange(`A${tierFirstRow}:E${tierLastRow}`).values = tierRows;
  for (let row = tierFirstRow; row <= tierLastRow; row += 1) {
    analysis.getRange(`A${row}:E${row}`).format.fill = row % 2 === 1 ? "#F7F5FB" : "#FFFFFF";
  }
  analysis.getRange(`B${tierFirstRow}:D${tierLastRow}`).format.numberFormat = "0.00%";
  analysis.getRange(`E${tierFirstRow}:E${tierLastRow}`).format.wrapText = true;
} else {
  analysis.getRange(`A${tierFirstRow}:E${tierFirstRow}`).merge();
  analysis.getRange(`A${tierFirstRow}`).values = [["No tier thresholds configured for this character."]];
}

analysis.getRange(`G${tierSectionRow}:N${tierSectionRow}`).merge();
analysis.getRange(`G${tierSectionRow}`).values = [["Interpretation and limits"]];
analysis.getRange(`G${tierSectionRow}:N${tierSectionRow}`).format = { fill: "#433B61", font: { bold: true, color: "#FFFFFF", fontSize: 11 } };
analysis.getRange(`G${tierHeaderRow}:N${tierLastRow}`).merge();
analysis.getRange(`G${tierHeaderRow}`).values = [[analysisData.presentation.analysis_note]];
analysis.getRange(`G${tierHeaderRow}:N${tierLastRow}`).format = {
  fill: "#F7F5FB", font: { color: "#433B61", fontSize: 10 }, wrapText: true,
  verticalAlignment: "top", borders: { preset: "outside", style: "thin", color: "#CFC7DF" },
};

const helperHeaderRow = tierHeaderRow;
const helperFirstRow = helperHeaderRow + 1;
const helperLastRow = helperFirstRow + abilityRows.length - 1;
analysis.getRange(`Q${helperHeaderRow}:T${helperHeaderRow}`).values = [["Ability", "First roll", "1 reroll", "2 rerolls"]];
for (let index = 0; index < abilityRows.length; index += 1) {
  const sourceRow = abilityFirstRow + index;
  const helperRow = helperFirstRow + index;
  analysis.getRange(`Q${helperRow}:T${helperRow}`).formulas = [[`=B${sourceRow}`, `=F${sourceRow}`, `=G${sourceRow}`, `=H${sourceRow}`]];
}
analysis.getRange(`R${helperFirstRow}:T${helperLastRow}`).format.numberFormat = "0%";
const chart = analysis.charts.add("bar", analysis.getRange(`Q${helperHeaderRow}:T${helperLastRow}`));
chart.title = "Ability qualification odds improve with rerolls";
chart.hasLegend = true;
chart.xAxis = { numberFormatCode: "0%", min: 0, max: 1 };
chart.yAxis = { numberFormatCode: "0%", min: 0, max: 1 };
chart.setPosition("P2", "W19");
for (const [column, width] of [
  ["A:A", 8], ["B:B", 18], ["C:C", 17], ["D:D", 28], ["E:E", 47],
  ["F:H", 12], ["I:I", 18], ["J:J", 22], ["K:K", 22], ["L:L", 54],
  ["M:M", 10], ["N:N", 70], ["O:O", 3], ["P:W", 12],
]) analysis.getRange(column).format.columnWidth = width;

const overall = workbook.worksheets.add(overallSheetName);
overall.showGridLines = false;
overall.freezePanes.freezeRows(8);
overall.getRange("A1:K1").merge();
overall.getRange("A1").values = [[`${characterName} Overall Value Cycle`]];
overall.getRange("A1:K1").format = {
  fill: "#29263D",
  font: { bold: true, color: "#FFFFFF", typeface: "Aptos Display", fontSize: 18 },
  verticalAlignment: "center",
};
overall.getRange("A1:K1").format.rowHeight = 32;
overall.getRange("A2:K2").merge();
overall.getRange("A2").values = [[
  analysisData.presentation.overall_note,
]];
overall.getRange("A2:K2").format = {
  fill: "#EDE9F6",
  font: { color: "#433B61", italic: true, fontSize: 10 },
  wrapText: true,
};

overall.getRange("A4:B4").merge();
overall.getRange("A4").values = [["Teacher success-first"]];
overall.getRange("D4:E4").merge();
overall.getRange("D4").values = [["Value optimized"]];
overall.getRange("G4:H4").merge();
overall.getRange("G4").values = [["Value tradeoff"]];
overall.getRange("J4:K4").merge();
overall.getRange("J4").values = [["Hit-rate tradeoff"]];
for (const range of ["A4:B4", "D4:E4", "G4:H4", "J4:K4"]) {
  overall.getRange(range).format = {
    fill: "#8C79B8",
    font: { bold: true, color: "#FFFFFF" },
    horizontalAlignment: "center",
  };
}
overall.getRange("A5").values = [["Hit chance"]];
overall.getRange("B5").formulas = [["=1-B9"]];
overall.getRange("D5").values = [["Hit chance"]];
overall.getRange("E5").formulas = [["=1-H9"]];
overall.getRange("G5").values = [["Average value gain"]];
overall.getRange("J5").values = [["Hit-rate change"]];
overall.getRange("K5").formulas = [["=(1-H9)-(1-B9)"]];
overall.getRange("B5:E5").format.numberFormat = "0.00%";
overall.getRange("H5").format.numberFormat = "0.00";
overall.getRange("K5").format.numberFormat = "0.00%";
for (const range of ["A5:B5", "D5:E5", "G5:H5", "J5:K5"]) {
  overall.getRange(range).format = {
    fill: "#F7F5FB",
    font: { bold: true, color: "#29263D" },
    borders: { preset: "outside", style: "thin", color: "#CFC7DF" },
  };
}

overall.getRange("A7:E7").merge();
overall.getRange("A7").values = [["Teacher success-first policy"]];
overall.getRange("G7:K7").merge();
overall.getRange("G7").values = [["Maximum expected-value policy"]];
for (const range of ["A7:E7", "G7:K7"]) {
  overall.getRange(range).format = {
    fill: "#433B61",
    font: { bold: true, color: "#FFFFFF", fontSize: 11 },
  };
}
const overallHeaders = ["Outcome", "Probability", "Listed value", "Avg contribution", "Interpretation"];
overall.getRange("A8:E8").values = [overallHeaders];
overall.getRange("G8:K8").values = [overallHeaders];
const teacherPolicy = analysisData.overall_policies.teacher_success_policy;
const valuePolicy = analysisData.overall_policies.value_optimized_policy
  ?? analysisData.overall_policies.damage_optimized_policy;
const policyRows = (policy) => policy.outcomes.map((row) => [
  row.label,
  row.probability,
  row.value ?? row.damage,
  row.average_value_contribution ?? row.average_damage_contribution,
  row.note,
]);
const overallFirstRow = 9;
const overallLastRow = overallFirstRow + teacherPolicy.outcomes.length - 1;
const overallTotalRow = overallLastRow + 1;
const overallAdjustedRow = overallLastRow + 2;
overall.getRange(`A${overallFirstRow}:E${overallLastRow}`).values = policyRows(teacherPolicy);
overall.getRange(`G${overallFirstRow}:K${overallLastRow}`).values = policyRows(valuePolicy);
overall.getRange(`A${overallTotalRow}:E${overallTotalRow}`).values = [["Total", null, null, null, `Gross ${valueLabel}`]];
overall.getRange(`G${overallTotalRow}:K${overallTotalRow}`).values = [["Total", null, null, null, `Gross ${valueLabel}`]];
overall.getRange(`B${overallTotalRow}`).formulas = [[`=SUM(B${overallFirstRow}:B${overallLastRow})`]];
overall.getRange(`D${overallTotalRow}`).formulas = [[`=SUM(D${overallFirstRow}:D${overallLastRow})`]];
overall.getRange(`H${overallTotalRow}`).formulas = [[`=SUM(H${overallFirstRow}:H${overallLastRow})`]];
overall.getRange(`J${overallTotalRow}`).formulas = [[`=SUM(J${overallFirstRow}:J${overallLastRow})`]];
overall.getRange("H5").formulas = [[`=J${overallTotalRow}-D${overallTotalRow}`]];
const adjustment = analysisData.presentation.adjustments?.[0];
const adjustmentLabel = adjustment?.label ?? "Adjusted total";
const adjustmentNote = adjustment?.note ?? "No configured adjustment";
const adjustedFormula = (totalCell, probabilityColumn, outcomes) => {
  if (!adjustment) return `=${totalCell}`;
  const index = outcomes.findIndex((outcome) => outcome.outcome === adjustment.outcome);
  if (index < 0) return `=${totalCell}`;
  const probabilityCell = `${probabilityColumn}${overallFirstRow + index}`;
  const operator = adjustment.operation === "add" ? "+" : "-";
  return `=${totalCell}${operator}(${probabilityCell}*${Number(adjustment.amount)})`;
};
overall.getRange(`A${overallAdjustedRow}:E${overallAdjustedRow}`).values = [[adjustmentLabel, null, null, null, adjustmentNote]];
overall.getRange(`G${overallAdjustedRow}:K${overallAdjustedRow}`).values = [[adjustmentLabel, null, null, null, adjustmentNote]];
overall.getRange(`D${overallAdjustedRow}`).formulas = [[adjustedFormula(`D${overallTotalRow}`, "B", teacherPolicy.outcomes)]];
overall.getRange(`J${overallAdjustedRow}`).formulas = [[adjustedFormula(`J${overallTotalRow}`, "H", valuePolicy.outcomes)]];
overall.getRange(`B${overallFirstRow}:B${overallTotalRow}`).format.numberFormat = "0.00%";
overall.getRange(`H${overallFirstRow}:H${overallTotalRow}`).format.numberFormat = "0.00%";
overall.getRange(`C${overallFirstRow}:D${overallAdjustedRow}`).format.numberFormat = "0.00";
overall.getRange(`I${overallFirstRow}:J${overallAdjustedRow}`).format.numberFormat = "0.00";
for (const headerRange of ["A8:E8", "G8:K8"]) {
  overall.getRange(headerRange).format = {
    fill: "#433B61",
    font: { bold: true, color: "#FFFFFF", fontSize: 9 },
    wrapText: true,
  };
}
for (let row = overallFirstRow; row <= overallLastRow; row += 1) {
  const fill = row % 2 === 1 ? "#F7F5FB" : "#FFFFFF";
  overall.getRange(`A${row}:E${row}`).format.fill = fill;
  overall.getRange(`G${row}:K${row}`).format.fill = fill;
}
for (const range of [`A${overallTotalRow}:E${overallAdjustedRow}`, `G${overallTotalRow}:K${overallAdjustedRow}`]) {
  overall.getRange(range).format = {
    fill: "#EDE9F6",
    font: { bold: true, color: "#29263D" },
    borders: { top: { style: "thin", color: "#8C79B8" } },
  };
}
overall.getRange(`E${overallFirstRow}:E${overallAdjustedRow}`).format.wrapText = true;
overall.getRange(`K${overallFirstRow}:K${overallAdjustedRow}`).format.wrapText = true;
overall.getRange(`A${overallFirstRow}:K${overallAdjustedRow}`).format.verticalAlignment = "top";
overall.getRange(`A${overallFirstRow}:K${overallLastRow}`).format.rowHeight = 34;
const overallWidths = [
  ["A:A", 31], ["B:B", 13], ["C:C", 11], ["D:D", 17], ["E:E", 49],
  ["F:F", 3], ["G:G", 31], ["H:H", 13], ["I:I", 11], ["J:J", 17], ["K:K", 49],
];
for (const [column, width] of overallWidths) overall.getRange(column).format.columnWidth = width;

const rerollDamage = workbook.worksheets.add("Overall Value by Rerolls");
const teacherStages = analysisData.overall_by_rerolls.teacher_success_policy;
const valueStages = analysisData.overall_by_rerolls.value_optimized_policy
  ?? analysisData.overall_by_rerolls.damage_optimized_policy;
const firstRerollOutcomeRow = 13;
const lastRerollOutcomeRow = 12 + teacherStages["0"].outcomes.length;
const rerollTotalRow = lastRerollOutcomeRow + 1;
const rerollNetRow = lastRerollOutcomeRow + 2;
const rerollAdjustment = analysisData.presentation.adjustments?.[0];
const teacherAdjustmentIndex = rerollAdjustment
  ? teacherStages["0"].outcomes.findIndex((outcome) => outcome.outcome === rerollAdjustment.outcome)
  : -1;
const valueAdjustmentIndex = rerollAdjustment
  ? valueStages["0"].outcomes.findIndex((outcome) => outcome.outcome === rerollAdjustment.outcome)
  : -1;
rerollDamage.showGridLines = false;
rerollDamage.freezePanes.freezeRows(5);
rerollDamage.getRange("A1:Q1").merge();
rerollDamage.getRange("A1").values = [[`${characterName} Overall ${valueLabel} by Rerolls`]];
rerollDamage.getRange("A1:Q1").format = {
  fill: "#29263D",
  font: { bold: true, color: "#FFFFFF", typeface: "Aptos Display", fontSize: 18 },
  verticalAlignment: "center",
};
rerollDamage.getRange("A1:Q1").format.rowHeight = 32;
rerollDamage.getRange("A2:Q2").merge();
rerollDamage.getRange("A2").values = [[
  analysisData.presentation.reroll_note,
]];
rerollDamage.getRange("A2:Q2").format = {
  fill: "#EDE9F6",
  font: { color: "#433B61", italic: true, fontSize: 10 },
  wrapText: true,
  verticalAlignment: "center",
};
rerollDamage.getRange("A2:Q2").format.rowHeight = 30;

for (const [range, label] of [
  ["A4:D4", "Reliability-first all-ability policy"],
  ["J4:M4", "Maximum expected-value policy"],
]) {
  rerollDamage.getRange(range).merge();
  rerollDamage.getRange(range.split(":")[0]).values = [[label]];
  rerollDamage.getRange(range).format = {
    fill: "#8C79B8",
    font: { bold: true, color: "#FFFFFF", fontSize: 10 },
    horizontalAlignment: "center",
  };
}
rerollDamage.getRange("A5:D5").values = [["Metric", "No rerolls", "1 reroll", "2 rerolls"]];
rerollDamage.getRange("J5:M5").values = [["Metric", "No rerolls", "1 reroll", "2 rerolls"]];
for (const range of ["A5:D5", "J5:M5"]) {
  rerollDamage.getRange(range).format = {
    fill: "#433B61",
    font: { bold: true, color: "#FFFFFF", fontSize: 9 },
    wrapText: true,
  };
}
const rerollAdjustedLabel = rerollAdjustment?.label ?? "Adjusted total";
rerollDamage.getRange("A6:A8").values = [["Ability hit chance"], [`Average ${valueLabel}`], [rerollAdjustedLabel]];
rerollDamage.getRange("J6:J8").values = [["Ability hit chance"], [`Average ${valueLabel}`], [rerollAdjustedLabel]];
rerollDamage.getRange("B6:D6").formulas = [[`=1-C${firstRerollOutcomeRow}`, `=1-D${firstRerollOutcomeRow}`, `=1-E${firstRerollOutcomeRow}`]];
rerollDamage.getRange("B7:D7").formulas = [[`=F${rerollTotalRow}`, `=G${rerollTotalRow}`, `=H${rerollTotalRow}`]];
rerollDamage.getRange("B8:D8").formulas = [[`=F${rerollNetRow}`, `=G${rerollNetRow}`, `=H${rerollNetRow}`]];
rerollDamage.getRange("K6:M6").formulas = [[`=1-L${firstRerollOutcomeRow}`, `=1-M${firstRerollOutcomeRow}`, `=1-N${firstRerollOutcomeRow}`]];
rerollDamage.getRange("K7:M7").formulas = [[`=O${rerollTotalRow}`, `=P${rerollTotalRow}`, `=Q${rerollTotalRow}`]];
rerollDamage.getRange("K8:M8").formulas = [[`=O${rerollNetRow}`, `=P${rerollNetRow}`, `=Q${rerollNetRow}`]];
rerollDamage.getRange("B6:D6").format.numberFormat = "0.00%";
rerollDamage.getRange("K6:M6").format.numberFormat = "0.00%";
rerollDamage.getRange("B7:D8").format.numberFormat = "0.0000";
rerollDamage.getRange("K7:M8").format.numberFormat = "0.0000";
for (const range of ["A6:D8", "J6:M8"]) {
  rerollDamage.getRange(range).format = {
    fill: "#F7F5FB",
    font: { color: "#29263D", fontSize: 10 },
    borders: { preset: "outside", style: "thin", color: "#CFC7DF" },
  };
}
rerollDamage.getRange("A6:A8").format.font = { bold: true, color: "#29263D" };
rerollDamage.getRange("J6:J8").format.font = { bold: true, color: "#29263D" };

for (const [range, label] of [
  ["A11:H11", "Reliability-first mutually exclusive ability outcomes"],
  ["J11:Q11", "Value-optimized mutually exclusive ability outcomes"],
]) {
  rerollDamage.getRange(range).merge();
  rerollDamage.getRange(range.split(":")[0]).values = [[label]];
  rerollDamage.getRange(range).format = {
    fill: "#433B61",
    font: { bold: true, color: "#FFFFFF", fontSize: 11 },
  };
}
const rerollHeaders = [
  "Outcome", "Listed value", "No reroll probability", "1 reroll probability", "2 reroll probability",
  "No reroll contribution", "1 reroll contribution", "2 reroll contribution",
];
rerollDamage.getRange("A12:H12").values = [rerollHeaders];
rerollDamage.getRange("J12:Q12").values = [rerollHeaders];
for (const range of ["A12:H12", "J12:Q12"]) {
  rerollDamage.getRange(range).format = {
    fill: "#433B61",
    font: { bold: true, color: "#FFFFFF", fontSize: 9 },
    wrapText: true,
    verticalAlignment: "center",
  };
}
rerollDamage.getRange("A12:Q12").format.rowHeight = 36;

const stagedRows = (stages) => stages["0"].outcomes.map((outcome, index) => [
  outcome.label,
  outcome.value ?? outcome.damage,
  stages["0"].outcomes[index].probability,
  stages["1"].outcomes[index].probability,
  stages["2"].outcomes[index].probability,
  null,
  null,
  null,
]);
rerollDamage.getRange(`A${firstRerollOutcomeRow}:H${lastRerollOutcomeRow}`).values = stagedRows(teacherStages);
rerollDamage.getRange(`J${firstRerollOutcomeRow}:Q${lastRerollOutcomeRow}`).values = stagedRows(valueStages);
for (let row = firstRerollOutcomeRow; row <= lastRerollOutcomeRow; row += 1) {
  rerollDamage.getRange(`F${row}:H${row}`).formulas = [[`=B${row}*C${row}`, `=B${row}*D${row}`, `=B${row}*E${row}`]];
  rerollDamage.getRange(`O${row}:Q${row}`).formulas = [[`=K${row}*L${row}`, `=K${row}*M${row}`, `=K${row}*N${row}`]];
}
const teacherRerollTable = rerollDamage.tables.add(`A12:H${lastRerollOutcomeRow}`, true, `${safeTablePrefix}TeacherValueByRerollsTable`);
teacherRerollTable.style = "TableStyleLight1";
const optimizedRerollTable = rerollDamage.tables.add(`J12:Q${lastRerollOutcomeRow}`, true, `${safeTablePrefix}OptimizedValueByRerollsTable`);
optimizedRerollTable.style = "TableStyleLight1";
rerollDamage.getRange(`A${rerollTotalRow}:H${rerollTotalRow}`).values = [["Total", null, null, null, null, null, null, null]];
rerollDamage.getRange(`J${rerollTotalRow}:Q${rerollTotalRow}`).values = [["Total", null, null, null, null, null, null, null]];
rerollDamage.getRange(`C${rerollTotalRow}:H${rerollTotalRow}`).formulas = [[
  `=SUM(C${firstRerollOutcomeRow}:C${lastRerollOutcomeRow})`,
  `=SUM(D${firstRerollOutcomeRow}:D${lastRerollOutcomeRow})`,
  `=SUM(E${firstRerollOutcomeRow}:E${lastRerollOutcomeRow})`,
  `=SUM(F${firstRerollOutcomeRow}:F${lastRerollOutcomeRow})`,
  `=SUM(G${firstRerollOutcomeRow}:G${lastRerollOutcomeRow})`,
  `=SUM(H${firstRerollOutcomeRow}:H${lastRerollOutcomeRow})`,
]];
rerollDamage.getRange(`L${rerollTotalRow}:Q${rerollTotalRow}`).formulas = [[
  `=SUM(L${firstRerollOutcomeRow}:L${lastRerollOutcomeRow})`,
  `=SUM(M${firstRerollOutcomeRow}:M${lastRerollOutcomeRow})`,
  `=SUM(N${firstRerollOutcomeRow}:N${lastRerollOutcomeRow})`,
  `=SUM(O${firstRerollOutcomeRow}:O${lastRerollOutcomeRow})`,
  `=SUM(P${firstRerollOutcomeRow}:P${lastRerollOutcomeRow})`,
  `=SUM(Q${firstRerollOutcomeRow}:Q${lastRerollOutcomeRow})`,
]];
rerollDamage.getRange(`A${rerollNetRow}:H${rerollNetRow}`).values = [[rerollAdjustedLabel, null, null, null, null, null, null, null]];
rerollDamage.getRange(`J${rerollNetRow}:Q${rerollNetRow}`).values = [[rerollAdjustedLabel, null, null, null, null, null, null, null]];
const rerollAdjustedFormula = (totalCell, probabilityCell) => {
  if (!rerollAdjustment || !probabilityCell) return `=${totalCell}`;
  const operator = rerollAdjustment.operation === "add" ? "+" : "-";
  return `=${totalCell}${operator}(${probabilityCell}*${Number(rerollAdjustment.amount)})`;
};
const teacherAdjustmentRow = teacherAdjustmentIndex >= 0 ? firstRerollOutcomeRow + teacherAdjustmentIndex : null;
const valueAdjustmentRow = valueAdjustmentIndex >= 0 ? firstRerollOutcomeRow + valueAdjustmentIndex : null;
rerollDamage.getRange(`F${rerollNetRow}:H${rerollNetRow}`).formulas = [[
  rerollAdjustedFormula(`F${rerollTotalRow}`, teacherAdjustmentRow ? `C${teacherAdjustmentRow}` : null),
  rerollAdjustedFormula(`G${rerollTotalRow}`, teacherAdjustmentRow ? `D${teacherAdjustmentRow}` : null),
  rerollAdjustedFormula(`H${rerollTotalRow}`, teacherAdjustmentRow ? `E${teacherAdjustmentRow}` : null),
]];
rerollDamage.getRange(`O${rerollNetRow}:Q${rerollNetRow}`).formulas = [[
  rerollAdjustedFormula(`O${rerollTotalRow}`, valueAdjustmentRow ? `L${valueAdjustmentRow}` : null),
  rerollAdjustedFormula(`P${rerollTotalRow}`, valueAdjustmentRow ? `M${valueAdjustmentRow}` : null),
  rerollAdjustedFormula(`Q${rerollTotalRow}`, valueAdjustmentRow ? `N${valueAdjustmentRow}` : null),
]];
rerollDamage.getRange(`C${firstRerollOutcomeRow}:E${rerollTotalRow}`).format.numberFormat = "0.00%";
rerollDamage.getRange(`L${firstRerollOutcomeRow}:N${rerollTotalRow}`).format.numberFormat = "0.00%";
rerollDamage.getRange(`B${firstRerollOutcomeRow}:B${lastRerollOutcomeRow}`).format.numberFormat = "0.0";
rerollDamage.getRange(`K${firstRerollOutcomeRow}:K${lastRerollOutcomeRow}`).format.numberFormat = "0.0";
rerollDamage.getRange(`F${firstRerollOutcomeRow}:H${rerollNetRow}`).format.numberFormat = "0.0000";
rerollDamage.getRange(`O${firstRerollOutcomeRow}:Q${rerollNetRow}`).format.numberFormat = "0.0000";
for (const range of [`A${rerollTotalRow}:H${rerollNetRow}`, `J${rerollTotalRow}:Q${rerollNetRow}`]) {
  rerollDamage.getRange(range).format = {
    fill: "#EDE9F6",
    font: { bold: true, color: "#29263D" },
    borders: { top: { style: "thin", color: "#8C79B8" } },
  };
}
rerollDamage.getRange(`A${firstRerollOutcomeRow}:Q${rerollNetRow}`).format.verticalAlignment = "top";
rerollDamage.getRange(`A${firstRerollOutcomeRow}:A${rerollNetRow}`).format.wrapText = true;
rerollDamage.getRange(`J${firstRerollOutcomeRow}:J${rerollNetRow}`).format.wrapText = true;
const rerollDamageWidths = [
  ["A:A", 31], ["B:B", 11], ["C:E", 17], ["F:H", 18], ["I:I", 3],
  ["J:J", 31], ["K:K", 11], ["L:N", 17], ["O:Q", 18],
];
for (const [column, width] of rerollDamageWidths) rerollDamage.getRange(column).format.columnWidth = width;

const stateFrequencies = workbook.worksheets.add("State Frequencies by Roll");
stateFrequencies.showGridLines = false;
stateFrequencies.freezePanes.freezeRows(5);
stateFrequencies.freezePanes.freezeColumns(4);
stateFrequencies.getRange("A1:R1").merge();
stateFrequencies.getRange("A1").values = [[`${characterName} State Frequencies Through the Reroll Cycle`]];
stateFrequencies.getRange("A1:R1").format = {
  fill: "#29263D",
  font: { bold: true, color: "#FFFFFF", typeface: "Aptos Display", fontSize: 18 },
  verticalAlignment: "center",
};
stateFrequencies.getRange("A1:R1").format.rowHeight = 32;
stateFrequencies.getRange("A2:R2").merge();
stateFrequencies.getRange("A2").values = [[
  "Opening combinations count the ordered 5d6 results that collapse into each sorted state, out of 7,776. Reroll columns show the probability that the entire offensive cycle occupies that sorted state immediately after the indicated reroll.",
]];
stateFrequencies.getRange("A2:R2").format = {
  fill: "#EDE9F6",
  font: { color: "#433B61", italic: true, fontSize: 10 },
  wrapText: true,
  verticalAlignment: "center",
};
stateFrequencies.getRange("A2:R2").format.rowHeight = 30;
stateFrequencies.getRange("A3:R3").merge();
stateFrequencies.getRange("A3").values = [[
  "Reliability-first follows the Teacher tabs. Value-first follows the Value-First tabs. When exact optimal keeps tie, the same primary keep displayed on the corresponding compact policy tab is used.",
]];
stateFrequencies.getRange("A3:R3").format = {
  fill: "#F7F5FB",
  font: { color: "#433B61", fontSize: 10 },
  wrapText: true,
  borders: { bottom: { style: "thin", color: "#8C79B8" } },
};
stateFrequencies.getRange("A3:R3").format.rowHeight = 30;
stateFrequencies.getRange("A4:R4").merge();
stateFrequencies.getRange("A4").values = [[
  "252 unique sorted states • probability columns total 100% • per-10,000 columns total 10,000 • per-7,776 columns total 7,776",
]];
stateFrequencies.getRange("A4:R4").format = {
  fill: "#8C79B8",
  font: { bold: true, color: "#FFFFFF", fontSize: 10 },
};
const stateFrequencyHeaders = [
  "State key", "Rolled dice", "Face counts (1–6)", "Symbols", "Opening ordered combinations",
  "Opening probability", "Opening per 10,000", "Opening per 7,776", "Reliability after 1st reroll",
  "Reliability per 10,000", "Reliability per 7,776", "Reliability after 2nd reroll",
  "Reliability per 10,000", "Reliability per 7,776", "Value-first after 1st reroll",
  "Value-first per 10,000", "Value-first after 2nd reroll", "Value-first per 10,000",
];
stateFrequencies.getRange("A5:R5").values = [stateFrequencyHeaders];
stateFrequencies.getRange("A5:R5").format = {
  fill: "#433B61",
  font: { bold: true, color: "#FFFFFF", fontSize: 9 },
  wrapText: true,
  verticalAlignment: "center",
};
stateFrequencies.getRange("A5:R5").format.rowHeight = 42;
const frequencyRows = analysisData.state_frequencies.rows.map((row) => [
  row.state_key,
  row.rolled_dice,
  row.face_counts,
  row.symbols,
  row.opening_ordered_combinations,
  row.opening_probability,
  null,
  null,
  row.reliability_after_first_probability,
  null,
  null,
  row.reliability_after_second_probability,
  null,
  null,
  row.value_after_first_probability,
  null,
  row.value_after_second_probability,
  null,
]);
const firstFrequencyRow = 6;
const lastFrequencyRow = firstFrequencyRow + frequencyRows.length - 1;
const frequencyTotalRow = lastFrequencyRow + 1;
stateFrequencies.getRange(`A${firstFrequencyRow}:R${lastFrequencyRow}`).values = frequencyRows;
for (let row = firstFrequencyRow; row <= lastFrequencyRow; row += 1) {
  stateFrequencies.getRange(`G${row}`).formulas = [[`=F${row}*10000`]];
  stateFrequencies.getRange(`H${row}`).formulas = [[`=F${row}*7776`]];
  stateFrequencies.getRange(`J${row}`).formulas = [[`=I${row}*10000`]];
  stateFrequencies.getRange(`K${row}`).formulas = [[`=I${row}*7776`]];
  stateFrequencies.getRange(`M${row}`).formulas = [[`=L${row}*10000`]];
  stateFrequencies.getRange(`N${row}`).formulas = [[`=L${row}*7776`]];
  stateFrequencies.getRange(`P${row}`).formulas = [[`=O${row}*10000`]];
  stateFrequencies.getRange(`R${row}`).formulas = [[`=Q${row}*10000`]];
}
stateFrequencies.getRange(`I${firstFrequencyRow}:I${lastFrequencyRow}`).values = analysisData.state_frequencies.rows.map(
  (row) => [row.reliability_after_first_probability],
);
stateFrequencies.getRange(`L${firstFrequencyRow}:L${lastFrequencyRow}`).values = analysisData.state_frequencies.rows.map(
  (row) => [row.reliability_after_second_probability],
);
stateFrequencies.getRange(`O${firstFrequencyRow}:O${lastFrequencyRow}`).values = analysisData.state_frequencies.rows.map(
  (row) => [row.value_after_first_probability],
);
stateFrequencies.getRange(`Q${firstFrequencyRow}:Q${lastFrequencyRow}`).values = analysisData.state_frequencies.rows.map(
  (row) => [row.value_after_second_probability],
);
for (let row = firstFrequencyRow; row <= lastFrequencyRow; row += 1) {
  if ((row - firstFrequencyRow) % 2 === 1) {
    stateFrequencies.getRange(`A${row}:R${row}`).format.fill = "#F2F2F2";
  }
}
stateFrequencies.getRange(`A${frequencyTotalRow}:R${frequencyTotalRow}`).values = [[
  "Total", null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null, null,
]];
for (const column of ["E", "F", "G", "H", "I", "J", "K", "L", "M", "N", "O", "P", "Q", "R"]) {
  stateFrequencies.getRange(`${column}${frequencyTotalRow}`).formulas = [[
    `=SUM(${column}${firstFrequencyRow}:${column}${lastFrequencyRow})`,
  ]];
}
stateFrequencies.getRange(`F${firstFrequencyRow}:F${frequencyTotalRow}`).format.numberFormat = "0.0000%";
stateFrequencies.getRange(`I${firstFrequencyRow}:I${frequencyTotalRow}`).format.numberFormat = "0.0000%";
stateFrequencies.getRange(`L${firstFrequencyRow}:L${frequencyTotalRow}`).format.numberFormat = "0.0000%";
stateFrequencies.getRange(`O${firstFrequencyRow}:O${frequencyTotalRow}`).format.numberFormat = "0.0000%";
stateFrequencies.getRange(`Q${firstFrequencyRow}:Q${frequencyTotalRow}`).format.numberFormat = "0.0000%";
stateFrequencies.getRange(`E${firstFrequencyRow}:E${frequencyTotalRow}`).format.numberFormat = "0";
for (const column of ["G", "H", "J", "K", "M", "N", "P", "R"]) {
  stateFrequencies.getRange(`${column}${firstFrequencyRow}:${column}${frequencyTotalRow}`).format.numberFormat = "0.00";
}
stateFrequencies.getRange(`A${firstFrequencyRow}:R${lastFrequencyRow}`).format = {
  font: { color: "#29263D", typeface: "Aptos", fontSize: 9 },
  verticalAlignment: "top",
};
stateFrequencies.getRange(`D${firstFrequencyRow}:D${lastFrequencyRow}`).format.wrapText = true;
stateFrequencies.getRange(`A${firstFrequencyRow}:R${lastFrequencyRow}`).format.rowHeight = 25;
stateFrequencies.getRange(`A${frequencyTotalRow}:R${frequencyTotalRow}`).format = {
  fill: "#EDE9F6",
  font: { bold: true, color: "#29263D" },
  borders: { top: { style: "thin", color: "#8C79B8" } },
};
const stateFrequencyWidths = [
  ["A:A", 15], ["B:B", 18], ["C:C", 19], ["D:D", 27], ["E:E", 22],
  ["F:F", 18], ["G:H", 18], ["I:I", 22], ["J:K", 20], ["L:L", 22],
  ["M:N", 20], ["O:O", 22], ["P:P", 20], ["Q:Q", 22], ["R:R", 20],
];
for (const [column, width] of stateFrequencyWidths) stateFrequencies.getRange(column).format.columnWidth = width;

function addTeacherGuideSheet(sheetName, guide, stageDescription, options = {}) {
  const sheet = workbook.worksheets.add(sheetName);
  sheet.showGridLines = false;
  sheet.freezePanes.freezeRows(5);
  sheet.freezePanes.freezeColumns(2);

  sheet.getRange("A1:O1").merge();
  sheet.getRange("A1").values = [[`${sheetName}: Exact Keep / Reroll Policy`]];
  sheet.getRange("A1:O1").format = {
    fill: "#29263D",
    font: { bold: true, color: "#FFFFFF", typeface: "Aptos Display", fontSize: 18 },
    verticalAlignment: "center",
  };
  sheet.getRange("A1:O1").format.rowHeight = 32;

  sheet.getRange("A2:O2").merge();
  sheet.getRange("A2").values = [[
    `${stageDescription} ${options.policyDescription ?? analysisData.presentation.teacher_policy_description}`,
  ]];
  sheet.getRange("A2:O2").format = {
    fill: "#EDE9F6",
    font: { color: "#433B61", italic: true, fontSize: 10 },
    wrapText: true,
    verticalAlignment: "center",
  };
  sheet.getRange("A2:O2").format.rowHeight = 30;

  sheet.getRange("A3:O3").merge();
  sheet.getRange("A3").values = [[
    guide.rerolls_remaining === 2
      ? `Use State key for exact lookup. Follow Keep / Reroll now, then look up the new sorted roll on ${options.nextSheetName ?? "Teacher After Second Roll"}. ‘Likely finishing ability’ is a forecast because the optimal policy may pivot after the reroll.`
      : "Use State key for exact lookup. Follow Keep / Reroll now, then resolve the resulting final roll. ‘Likely finishing ability’ is a forecast across all possible final results.",
  ]];
  sheet.getRange("A3:O3").format = {
    fill: "#F7F5FB",
    font: { color: "#433B61", fontSize: 10 },
    wrapText: true,
    borders: { bottom: { style: "thin", color: "#8C79B8" } },
  };
  sheet.getRange("A3:O3").format.rowHeight = 30;

  sheet.getRange("A4:O4").merge();
  sheet.getRange("A4").values = [[
    `${guide.rows.length} unique sorted 5d6 states • ${guide.rerolls_remaining} reroll${guide.rerolls_remaining === 1 ? "" : "s"} remaining`,
  ]];
  sheet.getRange("A4:O4").format = {
    fill: "#8C79B8",
    font: { bold: true, color: "#FFFFFF", fontSize: 10 },
    horizontalAlignment: "left",
  };

  const guideHeaders = [
    "State key", "Rolled dice", "Face counts (1–6)", "Symbols", "Best ability already complete",
    "Keep these dice", "Kept symbols", "Reroll these dice", "Other equally optimal keeps",
    "Eventual hit chance", "Expected listed value", "Likely finishing ability", "Forecast probability",
    "Top final results (up to 3)", options.instructionHeader ?? "Teacher instruction",
  ];
  sheet.getRange("A5:O5").values = [guideHeaders];
  sheet.getRange("A5:O5").format = {
    fill: "#433B61",
    font: { bold: true, color: "#FFFFFF", fontSize: 9 },
    wrapText: true,
    verticalAlignment: "center",
  };
  sheet.getRange("A5:O5").format.rowHeight = 32;

  const rows = guide.rows.map((row) => [
    row.state_key,
    row.rolled_dice,
    row.face_counts,
    row.symbols,
    row.current_best_ability,
    row.primary_keep,
    row.keep_symbols,
    row.reroll,
    row.alternate_optimal_keeps,
    row.success_probability,
    row.expected_listed_value,
    row.most_likely_final_ability,
    row.most_likely_probability,
    row.top_routes,
    row.action,
  ]);
  const lastRow = 5 + rows.length;
  sheet.getRange(`A6:O${lastRow}`).values = rows;
  const tableName = options.tableName ?? (sheetName === "Teacher After First Roll"
    ? "TeacherAfterFirstRollTable"
    : "TeacherAfterSecondRollTable");
  const table = sheet.tables.add(`A5:O${lastRow}`, true, tableName);
  table.style = "TableStyleLight1";

  for (let row = 6; row <= lastRow; row += 1) {
    sheet.getRange(`A${row}:O${row}`).format.fill = row % 2 === 0 ? "#F7F5FB" : "#FFFFFF";
  }
  sheet.getRange(`J6:J${lastRow}`).format.numberFormat = "0.00%";
  sheet.getRange(`K6:K${lastRow}`).format.numberFormat = "0.00";
  sheet.getRange(`M6:M${lastRow}`).format.numberFormat = "0.00%";
  sheet.getRange(`A6:O${lastRow}`).format = {
    font: { color: "#29263D", typeface: "Aptos", fontSize: 9 },
    verticalAlignment: "top",
  };
  sheet.getRange(`D6:I${lastRow}`).format.wrapText = true;
  sheet.getRange(`L6:O${lastRow}`).format.wrapText = true;
  sheet.getRange(`A6:O${lastRow}`).format.rowHeight = 34;

  const guideWidths = [
    ["A:A", 15], ["B:B", 17], ["C:C", 19], ["D:D", 24], ["E:E", 28],
    ["F:F", 19], ["G:G", 24], ["H:H", 19], ["I:I", 28], ["J:J", 17],
    ["K:K", 19], ["L:L", 29], ["M:M", 18], ["N:N", 57], ["O:O", 58],
  ];
  for (const [column, width] of guideWidths) sheet.getRange(column).format.columnWidth = width;
  return sheet;
}

const firstRollGuide = addTeacherGuideSheet(
  "Teacher After First Roll",
  analysisData.teacher_state_guides.after_first_roll,
  "Use immediately after the opening roll, with two rerolls remaining.",
);
const secondRollGuide = addTeacherGuideSheet(
  "Teacher After Second Roll",
  analysisData.teacher_state_guides.after_second_roll,
  "Use after the first reroll, with one final reroll remaining.",
);
const valueFirstRollGuide = addTeacherGuideSheet(
  "Value-First After First Roll",
  analysisData.value_first_state_guides.after_first_roll,
  "Use immediately after the opening roll, with two rerolls remaining.",
  {
    policyDescription: analysisData.presentation.value_policy_description,
    nextSheetName: "Value-First After Second Roll",
    instructionHeader: "Value-first instruction",
    tableName: "ValueFirstAfterFirstRollTable",
  },
);
const valueSecondRollGuide = addTeacherGuideSheet(
  "Value-First After Second Roll",
  analysisData.value_first_state_guides.after_second_roll,
  "Use after the first reroll, with one final reroll remaining.",
  {
    policyDescription: analysisData.presentation.value_policy_description,
    instructionHeader: "Value-first instruction",
    tableName: "ValueFirstAfterSecondRollTable",
  },
);

function addAllActionsSheet(sheetName, guide, stageDescription) {
  const sheet = workbook.worksheets.add(sheetName);
  sheet.showGridLines = false;
  sheet.freezePanes.freezeRows(5);
  sheet.freezePanes.freezeColumns(3);

  sheet.getRange("A1:P1").merge();
  sheet.getRange("A1").values = [[`${sheetName}: Every Legal Keep / Reroll Choice`]];
  sheet.getRange("A1:P1").format = {
    fill: "#29263D",
    font: { bold: true, color: "#FFFFFF", typeface: "Aptos Display", fontSize: 18 },
    verticalAlignment: "center",
  };
  sheet.getRange("A1:P1").format.rowHeight = 32;

  sheet.getRange("A2:P2").merge();
  sheet.getRange("A2").values = [[
    `${stageDescription} Every legal subset of the rolled dice is shown, including actions rejected by the current reliability-first teacher.`,
  ]];
  sheet.getRange("A2:P2").format = {
    fill: "#EDE9F6",
    font: { color: "#433B61", italic: true, fontSize: 10 },
    wrapText: true,
    verticalAlignment: "center",
  };
  sheet.getRange("A2:P2").format.rowHeight = 28;

  sheet.getRange("A3:P3").merge();
  sheet.getRange("A3").values = [[
    `Reliability continuation means: take this row’s action now, then maximize ability-hit chance on later decisions and use ${valueLabel} only for exact ties. Value-first continuation maximizes ${valueLabel} on every later decision.`,
  ]];
  sheet.getRange("A3:P3").format = {
    fill: "#F7F5FB",
    font: { color: "#433B61", fontSize: 10 },
    wrapText: true,
    borders: { bottom: { style: "thin", color: "#8C79B8" } },
  };
  sheet.getRange("A3:P3").format.rowHeight = 34;

  sheet.getRange("A4:P4").merge();
  sheet.getRange("A4").values = [[
    `${guide.rows.length.toLocaleString()} legal actions across 252 unique rolls • ${guide.rerolls_remaining} reroll${guide.rerolls_remaining === 1 ? "" : "s"} remaining`,
  ]];
  sheet.getRange("A4:P4").format = {
    fill: "#8C79B8",
    font: { bold: true, color: "#FFFFFF", fontSize: 10 },
  };

  const headers = [
    "State key", "Rolled dice", "Symbols", "Keep", "Kept symbols", "Reroll", "Dice rerolled",
    "Ability hit chance — reliability continuation", "Hit chance sacrificed", "Expected listed value — reliability continuation",
    "Value change vs reliability selection", "Reliability-first selected?",
    "Expected listed value — value-first continuation", "Value lost from best",
    "Value-first selected?", "Policy classification",
  ];
  sheet.getRange("A5:P5").values = [headers];
  sheet.getRange("A5:P5").format = {
    fill: "#433B61",
    font: { bold: true, color: "#FFFFFF", fontSize: 9 },
    wrapText: true,
    verticalAlignment: "center",
  };
  sheet.getRange("A5:P5").format.rowHeight = 40;

  const rows = guide.rows.map((row) => [
    row.state_key,
    row.rolled_dice,
    row.symbols,
    row.keep,
    row.keep_symbols,
    row.reroll,
    row.reroll_count,
    row.hit_probability,
    row.hit_probability_loss,
    row.reliability_continuation_value,
    row.value_change_vs_reliability_selection,
    row.reliability_selected,
    row.value_first_continuation_value,
    row.value_loss_from_best,
    row.value_first_selected,
    row.note,
  ]);
  const lastRow = 5 + rows.length;
  sheet.getRange(`A6:P${lastRow}`).values = rows;
  const tableName = guide.rerolls_remaining === 2
    ? "AllActionsTwoRerollsTable"
    : "AllActionsOneRerollTable";
  const table = sheet.tables.add(`A5:P${lastRow}`, true, tableName);
  table.style = "TableStyleLight1";

  sheet.getRange(`A6:P${lastRow}`).format = {
    font: { color: "#29263D", typeface: "Aptos", fontSize: 9 },
    verticalAlignment: "top",
  };
  sheet.getRange(`C6:F${lastRow}`).format.wrapText = true;
  sheet.getRange(`P6:P${lastRow}`).format.wrapText = true;
  sheet.getRange(`G6:G${lastRow}`).format.numberFormat = "0";
  sheet.getRange(`H6:I${lastRow}`).format.numberFormat = "0.0000%";
  sheet.getRange(`J6:K${lastRow}`).format.numberFormat = "0.0000";
  sheet.getRange(`M6:N${lastRow}`).format.numberFormat = "0.0000";
  sheet.getRange(`A6:P${lastRow}`).format.rowHeight = 25;

  const widths = [
    ["A:A", 15], ["B:B", 18], ["C:C", 26], ["D:D", 18], ["E:E", 26],
    ["F:F", 18], ["G:G", 13], ["H:H", 17], ["I:I", 19], ["J:J", 27],
    ["K:K", 24], ["L:L", 20], ["M:M", 27], ["N:N", 19], ["O:O", 18], ["P:P", 39],
  ];
  for (const [column, width] of widths) sheet.getRange(column).format.columnWidth = width;
  return sheet;
}

const allActionsTwo = addAllActionsSheet(
  "All Actions - 2 Rerolls",
  analysisData.all_action_guides.two_rerolls,
  "Use after the opening roll.",
);
const allActionsOne = addAllActionsSheet(
  "All Actions - 1 Reroll",
  analysisData.all_action_guides.one_reroll,
  "Use after the first reroll.",
);

const analysisInspect = await workbook.inspect({
  kind: "table",
  sheetId: analysisSheetName,
  range: `A1:N${tierLastRow}`,
  include: "values,formulas",
  tableMaxRows: 30,
  tableMaxCols: 14,
  maxChars: 20000,
});
console.log(analysisInspect.ndjson);
const overallInspect = await workbook.inspect({
  kind: "table",
  sheetId: overallSheetName,
  range: `A1:K${overallAdjustedRow}`,
  include: "values,formulas",
  tableMaxRows: 22,
  tableMaxCols: 12,
  maxChars: 18000,
});
console.log(overallInspect.ndjson);
const rerollDamageInspect = await workbook.inspect({
  kind: "table",
  sheetId: "Overall Value by Rerolls",
  range: `A1:Q${rerollNetRow}`,
  include: "values,formulas",
  tableMaxRows: 26,
  tableMaxCols: 17,
  maxChars: 26000,
});
console.log(rerollDamageInspect.ndjson);
const stateFrequencyInspect = await workbook.inspect({
  kind: "table",
  sheetId: "State Frequencies by Roll",
  range: "A1:R16",
  include: "values,formulas",
  tableMaxRows: 18,
  tableMaxCols: 18,
  maxChars: 26000,
});
console.log(stateFrequencyInspect.ndjson);
const stateFrequencyTotalInspect = await workbook.inspect({
  kind: "table",
  sheetId: "State Frequencies by Roll",
  range: `A${frequencyTotalRow}:R${frequencyTotalRow}`,
  include: "values,formulas",
  tableMaxRows: 3,
  tableMaxCols: 18,
  maxChars: 5000,
});
console.log(stateFrequencyTotalInspect.ndjson);
for (const sheetName of [
  "Teacher After First Roll",
  "Teacher After Second Roll",
  "Value-First After First Roll",
  "Value-First After Second Roll",
]) {
  const guideInspect = await workbook.inspect({
    kind: "table",
    sheetId: sheetName,
    range: "A1:O16",
    include: "values,formulas",
    tableMaxRows: 18,
    tableMaxCols: 15,
    maxChars: 22000,
  });
  console.log(guideInspect.ndjson);
}
for (const [sheetName, guide] of [
  ["All Actions - 2 Rerolls", analysisData.all_action_guides.two_rerolls],
  ["All Actions - 1 Reroll", analysisData.all_action_guides.one_reroll],
]) {
  const allActionsInspect = await workbook.inspect({
    kind: "table",
    sheetId: sheetName,
    range: "A1:P14",
    include: "values,formulas",
    tableMaxRows: 16,
    tableMaxCols: 16,
    maxChars: 20000,
  });
  console.log(allActionsInspect.ndjson);
  const sampleStart = 6 + guide.rows.findIndex((row) => row.state_key === "2-3-4-4-4");
  const sampleCount = guide.rows.filter((row) => row.state_key === "2-3-4-4-4").length;
  const sampleInspect = await workbook.inspect({
    kind: "table",
    sheetId: sheetName,
    range: `A${sampleStart}:P${sampleStart + sampleCount - 1}`,
    include: "values,formulas",
    tableMaxRows: 35,
    tableMaxCols: 16,
    maxChars: 28000,
  });
  console.log(sampleInspect.ndjson);
}
const errors = await workbook.inspect({
  kind: "match",
  searchTerm: "#REF!|#DIV/0!|#VALUE!|#NAME\\?|#N/A",
  options: { useRegex: true, maxResults: 300 },
  summary: "final formula error scan",
});
console.log(errors.ndjson);

await fs.mkdir(previewDir, { recursive: true });
const analysisPreview = await workbook.render({
  sheetName: analysisSheetName,
  range: `A1:W${Math.max(tierLastRow, helperLastRow)}`,
  scale: 1,
  format: "png",
});
await fs.writeFile(`${previewDir}/analysis.png`, new Uint8Array(await analysisPreview.arrayBuffer()));
const overallPreview = await workbook.render({
  sheetName: overallSheetName,
  range: `A1:K${overallAdjustedRow}`,
  scale: 1.25,
  format: "png",
});
await fs.writeFile(`${previewDir}/overall-value.png`, new Uint8Array(await overallPreview.arrayBuffer()));
const rerollDamagePreview = await workbook.render({
  sheetName: "Overall Value by Rerolls",
  range: `A1:Q${rerollNetRow}`,
  scale: 1,
  format: "png",
});
await fs.writeFile(`${previewDir}/overall-value-by-rerolls.png`, new Uint8Array(await rerollDamagePreview.arrayBuffer()));
const stateFrequencyPreview = await workbook.render({
  sheetName: "State Frequencies by Roll",
  range: "A1:R18",
  scale: 0.9,
  format: "png",
});
await fs.writeFile(`${previewDir}/state-frequencies-by-roll.png`, new Uint8Array(await stateFrequencyPreview.arrayBuffer()));
const sourceRowNumbers = analysisData.source_rows.map((row) => row.csv_row);
const referenceStartRow = sourceRowNumbers.length ? Math.max(1, Math.min(...sourceRowNumbers) - 2) : 1;
const referenceEndRow = sourceRowNumbers.length ? Math.min(csvValues.length, Math.max(...sourceRowNumbers) + 2) : Math.min(csvValues.length, 30);
const referencePreview = await workbook.render({
  sheetName: "Reference",
  range: `A${referenceStartRow}:P${referenceEndRow}`,
  scale: 1,
  format: "png",
});
await fs.writeFile(`${previewDir}/reference-character.png`, new Uint8Array(await referencePreview.arrayBuffer()));
const firstRollGuidePreview = await workbook.render({
  sheetName: "Teacher After First Roll",
  range: "A1:O18",
  scale: 0.9,
  format: "png",
});
await fs.writeFile(`${previewDir}/teacher-after-first-roll.png`, new Uint8Array(await firstRollGuidePreview.arrayBuffer()));
const secondRollGuidePreview = await workbook.render({
  sheetName: "Teacher After Second Roll",
  range: "A1:O18",
  scale: 0.9,
  format: "png",
});
await fs.writeFile(`${previewDir}/teacher-after-second-roll.png`, new Uint8Array(await secondRollGuidePreview.arrayBuffer()));
const valueFirstRollGuidePreview = await workbook.render({
  sheetName: "Value-First After First Roll",
  range: "A1:O18",
  scale: 0.9,
  format: "png",
});
await fs.writeFile(`${previewDir}/value-first-after-first-roll.png`, new Uint8Array(await valueFirstRollGuidePreview.arrayBuffer()));
const valueSecondRollGuidePreview = await workbook.render({
  sheetName: "Value-First After Second Roll",
  range: "A1:O18",
  scale: 0.9,
  format: "png",
});
await fs.writeFile(`${previewDir}/value-first-after-second-roll.png`, new Uint8Array(await valueSecondRollGuidePreview.arrayBuffer()));
for (const [sheetName, guide, outputName] of [
  ["All Actions - 2 Rerolls", analysisData.all_action_guides.two_rerolls, "all-actions-two-rerolls.png"],
  ["All Actions - 1 Reroll", analysisData.all_action_guides.one_reroll, "all-actions-one-reroll.png"],
]) {
  const preview = await workbook.render({
    sheetName,
    range: "A1:P18",
    scale: 0.9,
    format: "png",
  });
  await fs.writeFile(`${previewDir}/${outputName}`, new Uint8Array(await preview.arrayBuffer()));
  const sampleStart = 6 + guide.rows.findIndex((row) => row.state_key === "2-3-4-4-4");
  const sampleCount = guide.rows.filter((row) => row.state_key === "2-3-4-4-4").length;
  const samplePreview = await workbook.render({
    sheetName,
    range: `A${sampleStart}:P${sampleStart + sampleCount - 1}`,
    scale: 1,
    format: "png",
  });
  await fs.writeFile(`${previewDir}/${outputName.replace(".png", "-sample.png")}`, new Uint8Array(await samplePreview.arrayBuffer()));
}

await fs.mkdir(outputPath.slice(0, outputPath.lastIndexOf("/")), { recursive: true });
const output = await SpreadsheetFile.exportXlsx(workbook);
await output.save(outputPath);
console.log(outputPath);
