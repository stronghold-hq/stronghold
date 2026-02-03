'use client';

interface TableHeaderProps {
  columns: string[];
}

export function TableHeader({ columns }: TableHeaderProps) {
  return (
    <thead>
      <tr className="border-b border-[#222]">
        {columns.map((column) => (
          <th
            key={column}
            className="text-left text-gray-500 text-sm font-medium py-3 px-4"
          >
            {column}
          </th>
        ))}
      </tr>
    </thead>
  );
}
