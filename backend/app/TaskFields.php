<?php

namespace App;

use Illuminate\Support\Facades\DB;

class TaskFields
{
    /** @return array<int, array{key: string, label: string, type: string, required: bool, options: array}> */
    public function definitions(int $companyId, string $category): array
    {
        return DB::table('task_field_definitions')
            ->where('company_id', $companyId)
            ->where('category', $category)
            ->orderBy('id')
            ->get(['key', 'label', 'type', 'required', 'options'])
            ->map(static fn (object $field): array => [
                'key' => $field->key,
                'label' => $field->label,
                'type' => $field->type,
                'required' => (bool) $field->required,
                'options' => json_decode($field->options, true) ?: [],
            ])->all();
    }

    /** @param array<int, array{key: string, label: string, type: string, required: bool, options: array}> $definitions */
    public function validate(array $definitions, array $values): ?string
    {
        $allowed = [];
        foreach ($definitions as $definition) {
            $allowed[$definition['key']] = $definition;
            $value = $values[$definition['key']] ?? null;
            if ($definition['required'] && ($value === null || $value === '' || (is_string($value) && trim($value) === ''))) {
                return 'Заполните поле «'.$definition['label'].'»';
            }
        }

        foreach ($values as $key => $value) {
            if (! isset($allowed[$key])) {
                return 'Неизвестное поле заявки';
            }
            $definition = $allowed[$key];
            if ($value === null) {
                continue;
            }
            switch ($definition['type']) {
                case 'text':
                case 'date':
                    if (! is_string($value) || mb_strlen($value) > 1000) {
                        return 'Некорректное значение поля';
                    }
                    if ($definition['type'] === 'date' && $value !== '') {
                        $date = \DateTimeImmutable::createFromFormat('!Y-m-d', $value);
                        if ($date === false || $date->format('Y-m-d') !== $value) {
                            return 'Некорректная дата';
                        }
                    }
                    break;
                case 'number':
                    if (! is_int($value) && ! is_float($value)) {
                        return 'Ожидается число';
                    }
                    break;
                case 'boolean':
                    if (! is_bool($value)) {
                        return 'Ожидается логическое значение';
                    }
                    break;
                case 'select':
                    if (! is_string($value) || ($value !== '' && ! in_array($value, $definition['options'], true))) {
                        return 'Недопустимый вариант';
                    }
                    break;
                default:
                    return 'Неизвестный тип поля';
            }
        }

        return null;
    }
}
